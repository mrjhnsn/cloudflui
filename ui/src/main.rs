//! cloudflui OpenTUI frontend: renders the five panes and drives the Go
//! backend (see `backend.rs`) that talks to Cloudflare.

mod backend;

use backend::{Backend, Inbound, BACKEND_EXITED};
use opentui::prelude::*;
use serde::Deserialize;
use serde_json::json;
use std::cell::RefCell;
use std::rc::Rc;

const ACCENT: Color = Color::hex(0xf6821f);
const SUCCESS: Color = Color::hex(0x2ecc71);
const TEXT: Color = Color::hex(0xe6e6e6);
const MUTED: Color = Color::hex(0x8a8a8a);
const DIM: Color = Color::hex(0x5a5a5a);

const PANE_COUNT: usize = 5;

const HELP: &str =
    "tab/1-5: switch  j/k: move  enter: select  n: new  p: proxy  d: delete  s: save  r: refresh  esc: back  q: quit";

#[derive(Deserialize, Clone)]
struct Zone {
    id: String,
    name: String,
    status: String,
}

#[derive(Deserialize, Clone)]
struct Record {
    id: String,
    #[serde(rename = "type")]
    rtype: String,
    name: String,
    content: String,
    ttl: i64,
    proxied: bool,
    comment: String,
}

#[derive(Deserialize, Clone)]
struct Profile {
    name: String,
    api_key: String,
    account_id: String,
}

#[derive(Deserialize, Clone, Default)]
struct ConfigInfo {
    default_profile: String,
    active_profile: String,
    profiles: Vec<Profile>,
}

#[derive(Deserialize, Clone)]
struct WorkerScript {
    id: String,
    created_on: String,
}

#[derive(Deserialize, Clone)]
struct Deployment {
    created_on: String,
    strategy: String,
}

#[derive(Clone)]
struct Widgets {
    tabs: TabSelect,
    pane: View,
    status: Text,

    zones_view: View,
    zones: Select,

    records_view: View,
    records_title: Text,
    records: Select,

    editor_view: View,
    editor_title: Text,
    editor_labels: Vec<Text>,
    editor_fields: Vec<Input>,
    editor_proxy: Text,

    config_view: View,
    config_list: Select,
    config_form: View,
    config_labels: Vec<Text>,
    config_fields: Vec<Input>,
    config_help: Text,

    workers_view: View,
    workers_title: Text,
    workers: Select,
}

struct State {
    active: usize,
    editing: bool,
    zones: Vec<Zone>,
    selected_zone: Option<Zone>,
    records: Vec<Record>,
    editor_zone: Option<Zone>,
    editor_record: Option<Record>,
    editor_field: usize,
    editor_proxied: bool,
    config_form: bool,
    config_field: usize,
    scripts: Vec<WorkerScript>,
    deployments: Vec<Deployment>,
    workers_script: Option<String>,
    account_id: String,
    config: ConfigInfo,
}

impl Default for State {
    fn default() -> Self {
        State {
            active: 0,
            editing: false,
            zones: Vec::new(),
            selected_zone: None,
            records: Vec::new(),
            editor_zone: None,
            editor_record: None,
            editor_field: 0,
            editor_proxied: true,
            config_form: false,
            config_field: 0,
            scripts: Vec::new(),
            deployments: Vec::new(),
            workers_script: None,
            account_id: String::new(),
            config: ConfigInfo::default(),
        }
    }
}

#[derive(Clone)]
struct Ui {
    app: App,
    backend: Backend,
    state: Rc<RefCell<State>>,
    widgets: Widgets,
}

fn main() -> opentui::Result<()> {
    let mut options = AppOptions::default();
    options.tab_focus = false;
    let app = App::new(options)?;

    let widgets = build_widgets(&app);

    let backend = match Backend::spawn(app.proxy()) {
        Ok(backend) => backend,
        Err(e) => {
            eprintln!("cloudflui-ui: {e}");
            std::process::exit(1);
        }
    };

    let ui = Ui {
        app: app.clone(),
        backend,
        state: Rc::new(RefCell::new(State::default())),
        widgets,
    };

    register_handlers(&ui);

    ui.set_active(0);
    ui.load_config();
    ui.load_zones();

    ui.app.run()
}

fn build_widgets(app: &App) -> Widgets {
    let root = app.root();

    let header = Text::new(app, "cloudflui").fg(ACCENT).bold();
    root.add(&header);

    let tabs = TabSelect::new(app).options(["Zones", "Records", "Editor", "Config", "Workers"]);
    root.add(&tabs);

    let pane = View::new(app).border(BorderStyle::Rounded).padding(1).grow(1.0);
    root.add(&pane);

    // Zones pane.
    let zones = Select::new(app).grow(1.0);
    let zones_view = View::new(app).grow(1.0).child(&zones);
    pane.add(&zones_view);

    // Records pane.
    let records_title = Text::new(app, "Zone: ");
    let records = Select::new(app).grow(1.0);
    let records_view = View::new(app).column().grow(1.0);
    records_view.add(&records_title).add(&records);
    pane.add(&records_view);

    // Editor pane.
    let editor_title = Text::new(app, "DNS Record").fg(ACCENT).bold();
    let editor_view = View::new(app).column().grow(1.0);
    editor_view.add(&editor_title);
    let labels = ["Name", "Type", "Content", "TTL", "Comment"];
    let mut editor_labels = Vec::new();
    let mut editor_fields = Vec::new();
    for label in labels {
        let text = Text::new(app, format!("{label}:")).fg(MUTED);
        let input = Input::new(app);
        let row = View::new(app).row().gap(1);
        row.add(&text).add(&input);
        editor_view.add(&row);
        editor_labels.push(text);
        editor_fields.push(input);
    }
    let editor_proxy = Text::new(app, "  Proxied: off").fg(MUTED);
    editor_view.add(&editor_proxy);
    pane.add(&editor_view);

    // Config pane.
    let config_title = Text::new(app, "Profiles").fg(ACCENT).bold();
    let config_list = Select::new(app).grow(1.0);
    let config_help = Text::new(app, "j/k: move  enter: edit  n: new  a: activate  m: default  d: delete  r: reload")
        .fg(MUTED);
    let config_view = View::new(app).column().grow(1.0);
    config_view.add(&config_title).add(&config_list).add(&config_help);

    let config_form = View::new(app).column().grow(1.0);
    let mut config_labels = Vec::new();
    let mut config_fields = Vec::new();
    for label in ["Name", "API Key", "Account ID"] {
        let text = Text::new(app, format!("{label}:")).fg(MUTED);
        let input = Input::new(app);
        let row = View::new(app).row().gap(1);
        row.add(&text).add(&input);
        config_form.add(&row);
        config_labels.push(text);
        config_fields.push(input);
    }
    config_form.visible(false);
    config_view.add(&config_form);
    pane.add(&config_view);

    // Workers pane.
    let workers_title = Text::new(app, "Workers Scripts").fg(ACCENT).bold();
    let workers = Select::new(app).grow(1.0);
    let workers_view = View::new(app).column().grow(1.0);
    workers_view.add(&workers_title).add(&workers);
    pane.add(&workers_view);

    let status = Text::new(app, "Loading...").fg(TEXT).bg(ACCENT);
    root.add(&status);

    let help = Text::new(app, HELP).fg(MUTED);
    root.add(&help);

    Widgets {
        tabs,
        pane,
        status,
        zones_view,
        zones,
        records_view,
        records_title,
        records,
        editor_view,
        editor_title,
        editor_labels,
        editor_fields,
        editor_proxy,
        config_view,
        config_list,
        config_form,
        config_labels,
        config_fields,
        config_help,
        workers_view,
        workers_title,
        workers,
    }
}

fn register_handlers(ui: &Ui) {
    // Backend responses.
    {
        let ui = ui.clone();
        let app = ui.app.clone();
        app.on_message(move |cx, msg: &Inbound| ui.deliver(cx, msg));
    }

    // Global keys.
    {
        let ui = ui.clone();
        let app = ui.app.clone();
        app.on_key(move |cx, key| ui.handle_key(cx, key));
    }

    // Tab switching.
    {
        let ui = ui.clone();
        let tabs = ui.widgets.tabs.clone();
        tabs.on_change(move |_cx, index, _opt| {
            if ui.active() != index {
                ui.set_active(index);
            }
        });
    }

    // Zones: enter selects a zone.
    {
        let ui = ui.clone();
        let zones = ui.widgets.zones.clone();
        zones.on_select(move |_cx, index, _opt| {
            let zone = ui.state.borrow().zones.get(index).cloned();
            if let Some(zone) = zone {
                ui.state.borrow_mut().selected_zone = Some(zone);
                ui.set_active(1);
                ui.load_records();
            }
        });
    }

    // Records: enter edits the record.
    {
        let ui = ui.clone();
        let records = ui.widgets.records.clone();
        records.on_select(move |_cx, index, _opt| {
            let (zone, record) = {
                let s = ui.state.borrow();
                (s.selected_zone.clone(), s.records.get(index).cloned())
            };
            if let (Some(zone), Some(record)) = (zone, record) {
                ui.open_editor(zone, Some(record));
            }
        });
    }

    // Workers: enter loads a script's deployments.
    {
        let ui = ui.clone();
        let workers = ui.widgets.workers.clone();
        workers.on_select(move |_cx, index, _opt| {
            let script = {
                let s = ui.state.borrow();
                if s.workers_script.is_some() {
                    return;
                }
                s.scripts.get(index).cloned()
            };
            if let Some(script) = script {
                ui.state.borrow_mut().workers_script = Some(script.id.clone());
                ui.load_deployments(script.id);
            }
        });
    }

    // Config: enter edits the profile.
    {
        let ui = ui.clone();
        let config_list = ui.widgets.config_list.clone();
        config_list.on_select(move |_cx, index, _opt| {
            let profile = ui.state.borrow().config.profiles.get(index).cloned();
            if let Some(profile) = profile {
                ui.open_config_form(Some(profile));
            }
        });
    }

    // Editor fields: enter finishes editing.
    for i in 0..5 {
        let ui = ui.clone();
        let field = ui.widgets.editor_fields[i].clone();
        field.on_submit(move |_cx, _value| ui.finish_editing());
    }

    // Config fields: enter finishes editing.
    for i in 0..3 {
        let ui = ui.clone();
        let field = ui.widgets.config_fields[i].clone();
        field.on_submit(move |_cx, _value| ui.finish_editing());
    }
}

impl Ui {
    fn active(&self) -> usize {
        self.state.borrow().active
    }

    fn is_editing(&self) -> bool {
        self.state.borrow().editing
    }

    fn set_status(&self, msg: impl Into<String>) {
        self.widgets.status.set_text(msg.into());
    }

    fn deliver(&self, cx: &mut Cx<'_>, msg: &Inbound) {
        if msg.id == BACKEND_EXITED {
            self.set_status("Backend process exited");
            return;
        }
        self.backend.deliver(cx, msg);
    }

    fn handle_key(&self, cx: &mut Cx<'_>, key: &KeyEvent) {
        if key.matches("q") && !self.is_editing() {
            cx.quit();
            return;
        }

        if self.is_editing() {
            if key.matches("escape") {
                self.finish_editing();
            }
            return;
        }

        if key.matches("tab") {
            self.set_active((self.active() + 1) % PANE_COUNT);
            return;
        }
        if let KeyCode::Char(c) = key.code {
            if matches!(c, '1'..='5') {
                self.set_active((c as u8 - b'1') as usize);
                return;
            }
        }
        if key.matches("escape") {
            self.on_escape();
            return;
        }

        match self.active() {
            0 => self.handle_zones_key(key),
            1 => self.handle_records_key(key),
            2 => self.handle_editor_key(key),
            3 => self.handle_config_key(key),
            4 => self.handle_workers_key(key),
            _ => {}
        }
    }

    fn handle_zones_key(&self, key: &KeyEvent) {
        match key.code {
            KeyCode::Char('r') => self.load_zones(),
            KeyCode::Char('g') => {
                self.widgets.zones.perform(SelectAction::First);
            }
            KeyCode::Char('G') => {
                self.widgets.zones.perform(SelectAction::Last);
            }
            _ => {}
        }
    }

    fn handle_records_key(&self, key: &KeyEvent) {
        match key.code {
            KeyCode::Char('r') => self.load_records(),
            KeyCode::Char('n') => {
                let zone = self.state.borrow().selected_zone.clone();
                if let Some(zone) = zone {
                    self.open_editor(zone, None);
                }
            }
            KeyCode::Char('p') => self.toggle_proxy(),
            KeyCode::Char('d') => self.delete_record(),
            KeyCode::Char('g') => {
                self.widgets.records.perform(SelectAction::First);
            }
            KeyCode::Char('G') => {
                self.widgets.records.perform(SelectAction::Last);
            }
            _ => {}
        }
    }

    fn handle_editor_key(&self, key: &KeyEvent) {
        match key.code {
            KeyCode::Enter => self.start_editing(),
            KeyCode::Char(' ') => self.toggle_proxy_editor(),
            KeyCode::Char('s') => self.save_record(),
            KeyCode::Up | KeyCode::Char('k') => {
                let field = self.state.borrow().editor_field;
                if field > 0 {
                    self.set_editor_field(field - 1);
                }
            }
            KeyCode::Down | KeyCode::Char('j') => {
                let field = self.state.borrow().editor_field;
                if field < 5 {
                    self.set_editor_field(field + 1);
                }
            }
            _ => {}
        }
    }

    fn handle_config_key(&self, key: &KeyEvent) {
        if self.state.borrow().config_form {
            match key.code {
                KeyCode::Enter => self.start_editing(),
                KeyCode::Char('s') => self.save_profile(),
                KeyCode::Up | KeyCode::Char('k') => {
                    let field = self.state.borrow().config_field;
                    if field > 0 {
                        self.set_config_field(field - 1);
                    }
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    let field = self.state.borrow().config_field;
                    if field < 2 {
                        self.set_config_field(field + 1);
                    }
                }
                _ => {}
            }
            return;
        }
        match key.code {
            KeyCode::Char('n') => self.open_config_form(None),
            KeyCode::Char('r') => self.load_config(),
            KeyCode::Char('d') => self.delete_profile(),
            KeyCode::Char('a') => self.activate_profile(),
            KeyCode::Char('m') => self.set_default_profile(),
            KeyCode::Char('g') => {
                self.widgets.config_list.perform(SelectAction::First);
            }
            KeyCode::Char('G') => {
                self.widgets.config_list.perform(SelectAction::Last);
            }
            _ => {}
        }
    }

    fn handle_workers_key(&self, key: &KeyEvent) {
        match key.code {
            KeyCode::Char('r') => {
                let script = self.state.borrow().workers_script.clone();
                match script {
                    Some(script) => self.load_deployments(script),
                    None => self.load_workers(),
                }
            }
            KeyCode::Char('g') => {
                self.widgets.workers.perform(SelectAction::First);
            }
            KeyCode::Char('G') => {
                self.widgets.workers.perform(SelectAction::Last);
            }
            _ => {}
        }
    }

    fn on_escape(&self) {
        match self.active() {
            1 => self.set_active(0),
            2 => self.set_active(1),
            3 => {
                if self.state.borrow().config_form {
                    self.close_config_form();
                }
            }
            4 => {
                if self.state.borrow().workers_script.is_some() {
                    self.state.borrow_mut().workers_script = None;
                    self.render_workers();
                }
            }
            _ => {}
        }
    }

    fn set_active(&self, index: usize) {
        {
            let mut s = self.state.borrow_mut();
            s.active = index;
            s.editing = false;
        }
        self.widgets.tabs.select(index);
        self.sync_panes();
    }

    fn sync_panes(&self) {
        let active = self.active();
        let w = &self.widgets;
        w.zones_view.visible(active == 0);
        w.records_view.visible(active == 1);
        w.editor_view.visible(active == 2);
        w.config_view.visible(active == 3);
        w.workers_view.visible(active == 4);

        let title = match active {
            0 => "Zones",
            1 => "Records",
            2 => "Editor",
            3 => "Config",
            _ => "Workers",
        };
        w.pane.title(title);

        match active {
            0 => self.app.focus(&w.zones),
            1 => self.app.focus(&w.records),
            4 => self.app.focus(&w.workers),
            3 => {
                if self.state.borrow().config_form {
                    self.app.blur();
                } else {
                    self.app.focus(&w.config_list);
                }
            }
            _ => self.app.blur(),
        }
    }

    fn finish_editing(&self) {
        self.state.borrow_mut().editing = false;
        self.app.blur();
    }

    fn start_editing(&self) {
        let active = self.active();
        if active == 2 {
            let field = self.state.borrow().editor_field;
            if field < 5 {
                self.app.focus(&self.widgets.editor_fields[field]);
                self.state.borrow_mut().editing = true;
            } else {
                self.toggle_proxy_editor();
            }
        } else if active == 3 && self.state.borrow().config_form {
            let field = self.state.borrow().config_field;
            self.app.focus(&self.widgets.config_fields[field]);
            self.state.borrow_mut().editing = true;
        }
    }

    fn set_editor_field(&self, field: usize) {
        self.state.borrow_mut().editor_field = field;
        self.render_editor_cursor();
    }

    fn set_config_field(&self, field: usize) {
        self.state.borrow_mut().config_field = field;
        self.render_config_cursor();
    }

    fn toggle_proxy_editor(&self) {
        {
            let mut s = self.state.borrow_mut();
            s.editor_proxied = !s.editor_proxied;
        }
        self.render_editor_cursor();
    }

    fn render_editor_cursor(&self) {
        let (field, proxied) = {
            let s = self.state.borrow();
            (s.editor_field, s.editor_proxied)
        };
        let labels = ["Name", "Type", "Content", "TTL", "Comment"];
        for (i, label) in labels.iter().enumerate() {
            if i == field {
                self.widgets.editor_labels[i]
                    .set_text(format!("> {label}:"))
                    .fg(ACCENT)
                    .attributes(Attributes::BOLD);
            } else {
                self.widgets.editor_labels[i]
                    .set_text(format!("{label}:"))
                    .fg(MUTED)
                    .attributes(Attributes::NONE);
            }
        }
        let state = if proxied { "on" } else { "off" };
        if field == 5 {
            self.widgets.editor_proxy
                .set_text(format!("> Proxied: {state}"))
                .fg(ACCENT)
                .attributes(Attributes::BOLD);
        } else {
            let color = if proxied { SUCCESS } else { DIM };
            self.widgets.editor_proxy
                .set_text(format!("  Proxied: {state}"))
                .fg(color)
                .attributes(Attributes::NONE);
        }
    }

    fn render_config_cursor(&self) {
        let field = self.state.borrow().config_field;
        let labels = ["Name", "API Key", "Account ID"];
        for (i, label) in labels.iter().enumerate() {
            if i == field {
                self.widgets.config_labels[i]
                    .set_text(format!("> {label}:"))
                    .fg(ACCENT)
                    .attributes(Attributes::BOLD);
            } else {
                self.widgets.config_labels[i]
                    .set_text(format!("{label}:"))
                    .fg(MUTED)
                    .attributes(Attributes::NONE);
            }
        }
    }

    fn open_editor(&self, zone: Zone, record: Option<Record>) {
        let (name, rtype, content, ttl, comment, proxied) = match &record {
            Some(r) => (
                r.name.clone(),
                r.rtype.clone(),
                r.content.clone(),
                r.ttl.to_string(),
                r.comment.clone(),
                r.proxied,
            ),
            None => (String::new(), "A".into(), String::new(), "1".into(), String::new(), true),
        };
        self.widgets.editor_fields[0].set_value(name);
        self.widgets.editor_fields[1].set_value(rtype);
        self.widgets.editor_fields[2].set_value(content);
        self.widgets.editor_fields[3].set_value(ttl);
        self.widgets.editor_fields[4].set_value(comment);
        let title = if record.is_some() { "Edit DNS Record" } else { "New DNS Record" };
        self.widgets.editor_title.set_text(format!("{title} — {}", zone.name));

        {
            let mut s = self.state.borrow_mut();
            s.editor_zone = Some(zone);
            s.editor_record = record;
            s.editor_proxied = proxied;
            s.editor_field = 0;
            s.editing = false;
        }
        self.render_editor_cursor();
        self.set_active(2);
    }

    fn open_config_form(&self, profile: Option<Profile>) {
        let (name, api_key, account_id) = match &profile {
            Some(p) => (p.name.clone(), p.api_key.clone(), p.account_id.clone()),
            None => (String::new(), String::new(), String::new()),
        };
        self.widgets.config_fields[0].set_value(name);
        self.widgets.config_fields[1].set_value(api_key);
        self.widgets.config_fields[2].set_value(account_id);
        {
            let mut s = self.state.borrow_mut();
            s.config_form = true;
            s.config_field = 0;
            s.editing = false;
        }
        self.widgets.config_list.visible(false);
        self.widgets.config_form.visible(true);
        self.widgets.config_help.set_text("j/k: move  enter: edit field  s: save  esc: back");
        self.render_config_cursor();
        self.app.blur();
    }

    fn close_config_form(&self) {
        {
            let mut s = self.state.borrow_mut();
            s.config_form = false;
            s.editing = false;
        }
        self.widgets.config_form.visible(false);
        self.widgets.config_list.visible(true);
        self.widgets.config_help
            .set_text("j/k: move  enter: edit  n: new  a: activate  m: default  d: delete  r: reload");
        self.app.blur();
        self.app.focus(&self.widgets.config_list);
    }

    fn load_zones(&self) {
        let this = self.clone();
        this.backend.clone().call("list_zones", json!({}), move |_cx, result| match result {
            Ok(value) => {
                let list: Vec<Zone> = serde_json::from_value(value).unwrap_or_default();
                let options = list
                    .iter()
                    .map(|zone| {
                        let dot = match zone.status.as_str() {
                            "active" => "●",
                            "pending" => "◌",
                            _ => "○",
                        };
                        SelectOption::new(format!("{dot}  {}", zone.name), "")
                    })
                    .collect::<Vec<_>>();
                this.state.borrow_mut().zones = list;
                this.widgets.zones.options(options);
            }
            Err(e) => this.set_status(e),
        });
    }

    fn load_records(&self) {
        let zone = match self.state.borrow().selected_zone.clone() {
            Some(zone) => zone,
            None => return,
        };
        self.widgets.records_title.set_text(format!("Zone: {}", zone.name));
        let this = self.clone();
        this.backend.clone().call("list_records", json!({ "zone_id": zone.id }), move |_cx, result| {
            match result {
                Ok(value) => {
                    let list: Vec<Record> = serde_json::from_value(value).unwrap_or_default();
                    let options = list
                        .iter()
                        .map(|r| {
                            let dot = if r.proxied { "●" } else { " " };
                            SelectOption::new(format!("{dot} {:<6} {:<40} {}", r.rtype, r.name, r.content), "")
                        })
                        .collect::<Vec<_>>();
                    this.state.borrow_mut().records = list;
                    this.widgets.records.options(options);
                }
                Err(e) => this.set_status(e),
            }
        });
    }

    fn load_workers(&self) {
        let account_id = self.state.borrow().account_id.clone();
        let this = self.clone();
        this.backend.clone().call("list_workers", json!({ "account_id": account_id }), move |_cx, result| {
            match result {
                Ok(value) => {
                    let list: Vec<WorkerScript> = serde_json::from_value(value).unwrap_or_default();
                    this.state.borrow_mut().scripts = list;
                    this.render_workers();
                }
                Err(e) => this.set_status(e),
            }
        });
    }

    fn load_deployments(&self, script: String) {
        let account_id = self.state.borrow().account_id.clone();
        let this = self.clone();
        this.backend.clone().call(
            "list_deployments",
            json!({ "account_id": account_id, "script": script }),
            move |_cx, result| match result {
                Ok(value) => {
                    let list: Vec<Deployment> = serde_json::from_value(value).unwrap_or_default();
                    this.state.borrow_mut().deployments = list;
                    this.render_workers();
                }
                Err(e) => this.set_status(e),
            },
        );
    }

    fn render_workers(&self) {
        let script = self.state.borrow().workers_script.clone();
        if let Some(name) = script {
            self.widgets.workers_title.set_text(format!("Deployments: {name}"));
            let options = self
                .state
                .borrow()
                .deployments
                .iter()
                .map(|d| SelectOption::new(format!("{}  {}", d.created_on, d.strategy), ""))
                .collect::<Vec<_>>();
            self.widgets.workers.options(options);
        } else {
            self.widgets.workers_title.set_text("Workers Scripts");
            let options = self
                .state
                .borrow()
                .scripts
                .iter()
                .map(|s| SelectOption::new(format!("{}  {}", s.created_on, s.id), ""))
                .collect::<Vec<_>>();
            self.widgets.workers.options(options);
        }
    }

    fn load_config(&self) {
        let this = self.clone();
        this.backend.clone().call("get_config", json!({}), move |_cx, result| match result {
            Ok(value) => {
                let config: ConfigInfo = serde_json::from_value(value).unwrap_or_default();
                let account_id = config
                    .profiles
                    .iter()
                    .find(|p| p.name == config.active_profile)
                    .map(|p| p.account_id.clone())
                    .unwrap_or_default();
                let active = config.active_profile.clone();
                let default = config.default_profile.clone();
                let options = config
                    .profiles
                    .iter()
                    .map(|p| {
                        let mut line = p.name.clone();
                        if p.name == active {
                            line.push_str("  (active)");
                        }
                        if p.name == default {
                            line.push_str("  (default)");
                        }
                        SelectOption::new(line, "")
                    })
                    .collect::<Vec<_>>();
                {
                    let mut s = this.state.borrow_mut();
                    s.config = config;
                    s.account_id = account_id;
                }
                this.widgets.config_list.options(options);
                this.set_status(format!("Profile: {active}"));
                this.load_workers();
            }
            Err(e) => this.set_status(e),
        });
    }

    fn toggle_proxy(&self) {
        let index = self.widgets.records.selected();
        let (zone, record) = {
            let s = self.state.borrow();
            (s.selected_zone.clone(), s.records.get(index).cloned())
        };
        let (zone, record) = match (zone, record) {
            (Some(zone), Some(record)) => (zone, record),
            _ => return,
        };
        let this = self.clone();
        this.backend.clone().call(
            "update_record",
            json!({
                "zone_id": zone.id,
                "record_id": record.id,
                "name": record.name,
                "type": record.rtype,
                "content": record.content,
                "ttl": record.ttl,
                "proxied": !record.proxied,
                "comment": record.comment,
            }),
            move |_cx, result| match result {
                Ok(_) => {
                    this.set_status("Record saved");
                    this.load_records();
                }
                Err(e) => this.set_status(format!("Record save failed: {e}")),
            },
        );
    }

    fn delete_record(&self) {
        let index = self.widgets.records.selected();
        let (zone, record) = {
            let s = self.state.borrow();
            (s.selected_zone.clone(), s.records.get(index).cloned())
        };
        let (zone, record) = match (zone, record) {
            (Some(zone), Some(record)) => (zone, record),
            _ => return,
        };
        let this = self.clone();
        this.backend.clone().call(
            "delete_record",
            json!({ "zone_id": zone.id, "record_id": record.id }),
            move |_cx, result| match result {
                Ok(_) => {
                    this.set_status("Record deleted");
                    this.load_records();
                }
                Err(e) => this.set_status(format!("Record delete failed: {e}")),
            },
        );
    }

    fn save_record(&self) {
        let (zone, record) = {
            let s = self.state.borrow();
            (s.editor_zone.clone(), s.editor_record.clone())
        };
        let zone = match zone {
            Some(zone) => zone,
            None => return,
        };
        let proxied = self.state.borrow().editor_proxied;
        let name = self.widgets.editor_fields[0].value();
        let rtype = self.widgets.editor_fields[1].value();
        let content = self.widgets.editor_fields[2].value();
        let ttl_text = self.widgets.editor_fields[3].value();
        let comment = self.widgets.editor_fields[4].value();

        let ttl: i64 = match ttl_text.trim().parse() {
            Ok(ttl) => ttl,
            Err(_) => {
                self.set_status("TTL must be a number");
                return;
            }
        };
        if name.is_empty() || content.is_empty() {
            self.set_status("Name and Content are required");
            return;
        }

        let (method, params) = match &record {
            None => (
                "create_record",
                json!({
                    "zone_id": zone.id,
                    "name": name,
                    "type": rtype,
                    "content": content,
                    "ttl": ttl,
                    "proxied": proxied,
                    "comment": comment,
                }),
            ),
            Some(r) => (
                "update_record",
                json!({
                    "zone_id": zone.id,
                    "record_id": r.id,
                    "name": name,
                    "type": rtype,
                    "content": content,
                    "ttl": ttl,
                    "proxied": proxied,
                    "comment": comment,
                }),
            ),
        };

        let this = self.clone();
        this.backend.clone().call(method, params, move |_cx, result| match result {
            Ok(_) => {
                this.set_status("Record saved");
                this.set_active(1);
                this.load_records();
            }
            Err(e) => this.set_status(format!("Record save failed: {e}")),
        });
    }

    fn save_profile(&self) {
        let name = self.widgets.config_fields[0].value();
        let api_key = self.widgets.config_fields[1].value();
        let account_id = self.widgets.config_fields[2].value();
        if name.is_empty() {
            self.set_status("Profile name is required");
            return;
        }
        let this = self.clone();
        this.backend.clone().call(
            "save_profile",
            json!({ "name": name, "api_key": api_key, "account_id": account_id }),
            move |_cx, result| match result {
                Ok(_) => {
                    this.close_config_form();
                    this.set_status("Config saved");
                    this.load_config();
                }
                Err(e) => this.set_status(format!("Config save failed: {e}")),
            },
        );
    }

    fn delete_profile(&self) {
        let index = self.widgets.config_list.selected();
        let name = self.state.borrow().config.profiles.get(index).map(|p| p.name.clone());
        let name = match name {
            Some(name) => name,
            None => return,
        };
        let this = self.clone();
        this.backend.clone().call("delete_profile", json!({ "name": name }), move |_cx, result| match result {
            Ok(_) => this.load_config(),
            Err(e) => this.set_status(format!("Config save failed: {e}")),
        });
    }

    fn activate_profile(&self) {
        let index = self.widgets.config_list.selected();
        let name = self.state.borrow().config.profiles.get(index).map(|p| p.name.clone());
        let name = match name {
            Some(name) => name,
            None => return,
        };
        let this = self.clone();
        this.backend.clone().call("activate_profile", json!({ "name": name }), move |_cx, result| match result {
            Ok(_) => {
                this.set_status(format!("Active profile: {name}"));
                this.load_config();
            }
            Err(e) => this.set_status(e),
        });
    }

    fn set_default_profile(&self) {
        let index = self.widgets.config_list.selected();
        let name = self.state.borrow().config.profiles.get(index).map(|p| p.name.clone());
        let name = match name {
            Some(name) => name,
            None => return,
        };
        let this = self.clone();
        this.backend.clone().call("set_default_profile", json!({ "name": name }), move |_cx, result| match result {
            Ok(_) => {
                this.set_status(format!("Default profile: {name}"));
                this.load_config();
            }
            Err(e) => this.set_status(e),
        });
    }
}
