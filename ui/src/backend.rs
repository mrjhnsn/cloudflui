//! Client for the Go backend. Spawns the `cloudflui` process and speaks
//! newline-delimited JSON-RPC over its stdin/stdout, delivering responses back
//! to the UI thread through an [`AppProxy`].
use opentui::prelude::{AppProxy, Cx};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::cell::RefCell;
use std::collections::HashMap;
use std::io::{BufRead, BufReader, Write};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::rc::Rc;
use std::sync::mpsc;

/// Sentinel id used to signal that the backend process exited.
pub const BACKEND_EXITED: u64 = u64::MAX;

/// A response delivered from the backend thread to the UI thread.
#[derive(Clone, Debug, Deserialize)]
pub struct Inbound {
    pub id: u64,
    pub result: Option<Value>,
    pub error: Option<String>,
}

#[derive(Serialize)]
struct Outbound {
    id: u64,
    method: String,
    params: Value,
}

type Callback = Box<dyn FnOnce(&mut Cx<'_>, Result<Value, String>)>;

#[derive(Default)]
struct Shared {
    next_id: u64,
    pending: HashMap<u64, Callback>,
}

/// A cloneable handle to the backend. Every clone shares the same request
/// queue and pending-callback table, so it is safe to capture in handlers.
#[derive(Clone)]
pub struct Backend {
    tx: mpsc::Sender<Outbound>,
    shared: Rc<RefCell<Shared>>,
}

impl Backend {
    /// Spawns the backend process and starts the reader thread.
    pub fn spawn(proxy: AppProxy) -> Result<Self, String> {
        let bin = backend_binary();
        let mut child = Command::new(&bin)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| format!("failed to start backend {}: {e}", bin.display()))?;
        let stdin = child.stdin.take().ok_or_else(|| "no stdin".to_string())?;
        let stdout = child.stdout.take().ok_or_else(|| "no stdout".to_string())?;
        let (tx, rx) = mpsc::channel::<Outbound>();

        std::thread::spawn(move || backend_loop(child, stdin, stdout, rx, proxy));

        Ok(Backend { tx, shared: Rc::new(RefCell::new(Shared::default())) })
    }

    /// Sends a request; `done` runs on the UI thread when the response arrives.
    pub fn call<F>(&self, method: &str, params: Value, done: F)
    where
        F: FnOnce(&mut Cx<'_>, Result<Value, String>) + 'static,
    {
        let id = {
            let mut shared = self.shared.borrow_mut();
            shared.next_id += 1;
            shared.next_id
        };
        self.shared.borrow_mut().pending.insert(id, Box::new(done));
        // If the backend thread is gone, drop the pending callback rather than
        // leak it; the UI learns of the exit through the BACKEND_EXITED event.
        let _ = self.tx.send(Outbound { id, method: method.to_string(), params });
    }

    /// Runs the callback registered for `msg`'s id, if any.
    pub fn deliver(&self, cx: &mut Cx<'_>, msg: &Inbound) {
        let done = self.shared.borrow_mut().pending.remove(&msg.id);
        if let Some(done) = done {
            let result = match (&msg.result, &msg.error) {
                (Some(value), _) => Ok(value.clone()),
                (_, Some(error)) => Err(error.clone()),
                _ => Err("empty response".to_string()),
            };
            done(cx, result);
        }
    }
}

fn backend_binary() -> PathBuf {
    if let Ok(path) = std::env::var("CLOUDFLUI_BACKEND") {
        if !path.is_empty() {
            return PathBuf::from(path);
        }
    }
    PathBuf::from("cloudflui")
}

fn backend_loop(
    mut child: Child,
    stdin: std::process::ChildStdin,
    stdout: std::process::ChildStdout,
    rx: mpsc::Receiver<Outbound>,
    proxy: AppProxy,
) {
    let mut writer = std::io::BufWriter::new(stdin);
    let mut reader = BufReader::new(stdout);

    for out in rx {
        let line = match serde_json::to_string(&out) {
            Ok(line) => line,
            Err(_) => continue,
        };
        if writer.write_all(line.as_bytes()).is_err()
            || writer.write_all(b"\n").is_err()
            || writer.flush().is_err()
        {
            break;
        }
        let mut response = String::new();
        match reader.read_line(&mut response) {
            Ok(0) => break,
            Ok(_) => {
                let inbound = match serde_json::from_str::<Inbound>(&response) {
                    Ok(value) => value,
                    Err(_) => Inbound {
                        id: out.id,
                        result: None,
                        error: Some("invalid backend response".to_string()),
                    },
                };
                let _ = proxy.send(inbound);
            }
            Err(_) => break,
        }
    }

    let _ = proxy.send(Inbound { id: BACKEND_EXITED, result: None, error: None });
    let _ = child.kill();
    let _ = child.wait();
}
