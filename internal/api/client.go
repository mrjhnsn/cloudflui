// Package api wraps the Cloudflare SDK with the operations cloudflui needs.
package api

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/workers"
	"github.com/cloudflare/cloudflare-go/v7/zones"
)

// Client wraps the Cloudflare SDK client.
type Client struct {
	cf *cloudflare.Client
}

// New creates a client authenticated with an API token.
func New(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("empty API token")
	}
	return &Client{cf: cloudflare.NewClient(option.WithAPIToken(apiKey))}, nil
}

// listPerPage is the page size requested from the Cloudflare API. Cloudflare
// defaults to 20 for zones, which silently truncates accounts with more.
const listPerPage = 50.0

// ListZones returns all zones for the account, following every page.
func (c *Client) ListZones(ctx context.Context) ([]zones.Zone, error) {
	iter := c.cf.Zones.ListAutoPaging(ctx, zones.ZoneListParams{
		PerPage: cloudflare.F(listPerPage),
	})
	var out []zones.Zone
	for iter.Next() {
		out = append(out, iter.Current())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListRecords returns all DNS records for a zone, following every page.
func (c *Client) ListRecords(ctx context.Context, zoneID string) ([]dns.RecordResponse, error) {
	iter := c.cf.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID:  cloudflare.F(zoneID),
		PerPage: cloudflare.F(listPerPage),
	})
	var out []dns.RecordResponse
	for iter.Next() {
		out = append(out, iter.Current())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateRecord creates a DNS record of a supported simple type.
func (c *Client) CreateRecord(ctx context.Context, zoneID, name, rtype, content string, ttl int, proxied bool, comment string) (*dns.RecordResponse, error) {
	body, err := buildRecordBody(rtype, name, content, ttl, proxied, comment)
	if err != nil {
		return nil, err
	}
	return c.cf.DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Body:   body,
	})
}

// UpdateRecord updates an existing DNS record.
func (c *Client) UpdateRecord(ctx context.Context, zoneID, recordID, name, rtype, content string, ttl int, proxied bool, comment string) (*dns.RecordResponse, error) {
	body, err := buildRecordBody(rtype, name, content, ttl, proxied, comment)
	if err != nil {
		return nil, err
	}
	updateBody, ok := body.(dns.RecordUpdateParamsBodyUnion)
	if !ok {
		return nil, fmt.Errorf("record type %q not supported for update", rtype)
	}
	return c.cf.DNS.Records.Update(ctx, recordID, dns.RecordUpdateParams{
		ZoneID: cloudflare.F(zoneID),
		Body:   updateBody,
	})
}

// DeleteRecord removes a DNS record.
func (c *Client) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	_, err := c.cf.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cloudflare.F(zoneID),
	})
	return err
}

// ListWorkers returns all Workers scripts for an account.
func (c *Client) ListWorkers(ctx context.Context, accountID string) ([]workers.ScriptListResponse, error) {
	page, err := c.cf.Workers.Scripts.List(ctx, workers.ScriptListParams{
		AccountID: cloudflare.F(accountID),
	})
	if err != nil {
		return nil, err
	}
	return page.Result, nil
}

// ListDeployments returns the deployment history for a Workers script.
func (c *Client) ListDeployments(ctx context.Context, accountID, scriptName string) ([]workers.Deployment, error) {
	resp, err := c.cf.Workers.Scripts.Deployments.List(ctx, scriptName, workers.ScriptDeploymentListParams{
		AccountID: cloudflare.F(accountID),
	})
	if err != nil {
		return nil, err
	}
	return resp.Deployments, nil
}

// buildRecordBody constructs the type-specific record body for the simple
// record types cloudflui supports (A, AAAA, CNAME, MX, TXT).
func buildRecordBody(rtype, name, content string, ttl int, proxied bool, comment string) (dns.RecordNewParamsBodyUnion, error) {
	ttlField := cloudflare.F(dns.TTL(ttl))
	nameField := cloudflare.F(name)
	contentField := cloudflare.F(content)
	commentField := cloudflare.F(comment)
	proxiedField := cloudflare.F(proxied)

	switch rtype {
	case "A":
		return dns.ARecordParam{
			Name:    nameField,
			TTL:     ttlField,
			Type:    cloudflare.F(dns.ARecordTypeA),
			Comment: commentField,
			Content: contentField,
			Proxied: proxiedField,
		}, nil
	case "AAAA":
		return dns.AAAARecordParam{
			Name:    nameField,
			TTL:     ttlField,
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Comment: commentField,
			Content: contentField,
			Proxied: proxiedField,
		}, nil
	case "CNAME":
		return dns.CNAMERecordParam{
			Name:    nameField,
			TTL:     ttlField,
			Type:    cloudflare.F(dns.CNAMERecordTypeCNAME),
			Comment: commentField,
			Content: contentField,
			Proxied: proxiedField,
		}, nil
	case "MX":
		return dns.MXRecordParam{
			Name:    nameField,
			TTL:     ttlField,
			Type:    cloudflare.F(dns.MXRecordTypeMX),
			Comment: commentField,
			Content: contentField,
			Proxied: proxiedField,
		}, nil
	case "TXT":
		return dns.TXTRecordParam{
			Name:    nameField,
			TTL:     ttlField,
			Type:    cloudflare.F(dns.TXTRecordTypeTXT),
			Comment: commentField,
			Content: contentField,
			Proxied: proxiedField,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported record type %q", rtype)
	}
}
