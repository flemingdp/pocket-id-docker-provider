package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Config describes runtime inputs. Apply=false means no Pocket ID writes.
type Config struct {
	BaseURL, APIKey, DockerSocket string
	Apply                         bool
}
type Container struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Labels map[string]string `json:"Labels"`
}
type Client struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	CallbackURLs      []string `json:"callbackURLs"`
	IsGroupRestricted bool     `json:"isGroupRestricted"`
	IsPublic          bool     `json:"isPublic"`
	PKCEEnabled       bool     `json:"pkceEnabled"`
}
type group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type page[T any] struct {
	Data       []T `json:"data"`
	Pagination struct {
		TotalPages int `json:"totalPages"`
	} `json:"pagination"`
}
type Desired struct {
	Client Client
	Groups []string
	Source string
}
// Summary records reconciliation counts. Client counts describe metadata, not REST calls.
type Summary struct {
 ContainersScanned int `json:"containers_scanned"`
 ClientsDiscovered int `json:"clients_discovered"`
 ClientsCreated int `json:"clients_created"`
 ClientsUpdated int `json:"clients_updated"`
 ClientsUnchanged int `json:"clients_unchanged"`
 InvalidLabels int `json:"invalid_labels"`
 Errors int `json:"errors"`
}
type Provider struct {
	cfg         Config
	docker, api *http.Client
	log         *slog.Logger
}

func New(cfg Config, logger *slog.Logger) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("POCKET_ID_URL is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New("invalid POCKET_ID_URL")
	}
	if cfg.Apply && cfg.APIKey == "" {
		return nil, errors.New("POCKET_ID_API_KEY is required with --apply")
	}
	if cfg.DockerSocket == "" {
		cfg.DockerSocket = "/var/run/docker.sock"
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", cfg.DockerSocket)
	}}
	return &Provider{cfg: cfg, log: logger, docker: &http.Client{Transport: transport, Timeout: 10 * time.Second}, api: &http.Client{Timeout: 15 * time.Second}}, nil
}
func (p *Provider) dockerContainers(ctx context.Context) ([]Container, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json", nil)
	resp, err := p.docker.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("docker status %d", resp.StatusCode)
	}
	var containers []Container
	return containers, json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&containers)
}
func enabled(v string) bool { return strings.EqualFold(v, "true") }
func parseLabels(c Container) (Desired, bool, error) {
	l := c.Labels
	if !enabled(l["pocketid.enable"]) {
		return Desired{}, false, nil
	}
	mode := l["pocketid.mode"]
	if mode == "" {
		mode = "oidc"
	}
	if mode != "oidc" && mode != "forwardauth" {
		return Desired{}, false, fmt.Errorf("invalid pocketid.mode %q", mode)
	}
	// Forward-auth requires a separate audited authentication gateway. Do not pretend to protect services.
	if mode == "forwardauth" {
		return Desired{}, false, nil
	}
	id := strings.TrimSpace(l["pocketid.client.id"])
	if id == "" {
		return Desired{}, false, errors.New("pocketid.client.id is required")
	}
	name := strings.TrimSpace(l["pocketid.client.name"])
	if name == "" {
		name = id
	}
	callback := strings.TrimSpace(l["pocketid.client.callback"])
	if callback == "" {
		return Desired{}, false, errors.New("pocketid.client.callback is required (also in forwardauth mode)")
	}
	u, err := url.Parse(callback)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Fragment != "" {
		return Desired{}, false, errors.New("callback must be an absolute HTTPS URL")
	}
	groups := []string{}
	for _, g := range strings.Split(l["pocketid.groups"], ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		return Desired{}, false, errors.New("pocketid.groups required: deny-by-default")
	}
	sort.Strings(groups)
	return Desired{Client: Client{ID: id, Name: name, CallbackURLs: []string{callback}, IsGroupRestricted: true, IsPublic: enabled(l["pocketid.client.public"]), PKCEEnabled: true}, Groups: groups, Source: c.ID}, true, nil
}
func (p *Provider) desired(ctx context.Context) ([]Desired, Summary, error) {
	cs, err := p.dockerContainers(ctx)
 summary := Summary{ContainersScanned:len(cs)}
	if err != nil {
		return nil, summary, err
	}
	byID := map[string]Desired{}
	for _, c := range cs {
		d, ok, err := parseLabels(c)
		if err != nil {
			p.log.Error("invalid container labels", "container", c.ID, "error", err)
 summary.InvalidLabels++
 summary.Errors++
			continue
		}
		if !ok {
			continue
		}
		if existing, exists := byID[d.Client.ID]; exists {
			summary.Errors++
 return nil, summary, fmt.Errorf("duplicate client %q on containers %s and %s", d.Client.ID, existing.Source, d.Source)
		}
		byID[d.Client.ID] = d
	}
	out := make([]Desired, 0, len(byID))
	for _, v := range byID {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Client.ID < out[j].Client.ID })
	summary.ClientsDiscovered = len(out)
 return out, summary, nil
}
func (p *Provider) request(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.cfg.BaseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", p.cfg.APIKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Pocket ID %s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
	}
	return nil
}
func listAll[T any](ctx context.Context, p *Provider, path string) ([]T, error) {
	all := []T{}
	for n := 1; n <= 1000; n++ {
		var data page[T]
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		err := p.request(ctx, "GET", fmt.Sprintf("%s%spagination%%5Bpage%%5D=%d&pagination%%5Blimit%%5D=100", path, sep, n), nil, &data)
		if err != nil {
			return nil, err
		}
		all = append(all, data.Data...)
		if n >= data.Pagination.TotalPages {
			return all, nil
		}
	}
	return nil, errors.New("pagination limit exceeded")
}
// Reconcile retains the error-only signature for existing callers.
func (p *Provider) Reconcile(ctx context.Context) error {
 _, err := p.ReconcileWithSummary(ctx)
 return err
}

// ReconcileWithSummary reconciles clients and produces counts for structured logs.
func (p *Provider) ReconcileWithSummary(ctx context.Context) (Summary,error) {
 desired, summary, err := p.desired(ctx)
 if err != nil { return summary,err }
 for _, d := range desired {
  p.log.Debug("discovered client","client",d.Client.ID,"callback",d.Client.CallbackURLs[0],"groups",d.Groups)
 }
 if !p.cfg.Apply {
  p.log.Debug("dry-run; no Pocket ID writes","count",len(desired))
  return summary,nil
 }
 if len(desired)==0 { return summary,nil }
 clients,err := listAll[Client](ctx,p,"/api/oidc/clients")
 if err != nil { summary.Errors++;return summary,err }
 groups,err := listAll[group](ctx,p,"/api/user-groups")
 if err != nil { summary.Errors++;return summary,err }
 clientMap:=map[string]Client{}
 for _,c:=range clients { clientMap[c.ID]=c }
 groupMap:=map[string]string{}
 for _,g:=range groups {groupMap[g.Name]=g.ID}
 var errs []error
 for _,d:=range desired {
  ids:=[]string{}
  missing:=false
  for _,name:=range d.Groups {
   if id,ok:=groupMap[name];ok {ids=append(ids,id)} else {
    p.log.Error("group not found; skipping client","client",d.Client.ID,"group",name)
    missing=true
   }
  }
  if missing {
   summary.Errors++
   errs=append(errs,fmt.Errorf("%s: missing group",d.Client.ID))
   continue
  }
  if previous,ok:=clientMap[d.Client.ID];ok {
   payload:=map[string]any{"name":d.Client.Name,"callbackURLs":d.Client.CallbackURLs,"isGroupRestricted":true}
   if previous.Name!=d.Client.Name || strings.Join(previous.CallbackURLs,"\x00")!=strings.Join(d.Client.CallbackURLs,"\x00") || !previous.IsGroupRestricted {
    if err:=p.request(ctx,"PATCH","/api/oidc/clients/"+url.PathEscape(d.Client.ID),payload,nil);err!=nil {
     summary.Errors++;errs=append(errs,fmt.Errorf("update %s: %w",d.Client.ID,err));continue
    }
    summary.ClientsUpdated++
    p.log.Info("updated client","client",d.Client.ID)
   } else {summary.ClientsUnchanged++}
  } else {
   var created map[string]any
   if err:=p.request(ctx,"POST","/api/oidc/clients",d.Client,&created);err!=nil {
    summary.Errors++;errs=append(errs,fmt.Errorf("create %s: %w",d.Client.ID,err));continue
   }
   if secret,ok:=created["createdSecret"];ok && secret!=nil {
    p.log.Warn("new client secret returned by API but intentionally not stored; rotate/retrieve via secure administrative workflow","client",d.Client.ID)
   }
   summary.ClientsCreated++
   p.log.Info("created client","client",d.Client.ID)
  }
  // Group membership PUT still happens on unchanged client metadata.
  if err:=p.request(ctx,"PUT","/api/oidc/clients/"+url.PathEscape(d.Client.ID)+"/allowed-user-groups",map[string]any{"userGroupIds":ids},nil);err!=nil {
   summary.Errors++;errs=append(errs,fmt.Errorf("synchronize groups for %s: %w",d.Client.ID,err));continue
  }
  p.log.Debug("synchronized groups","client",d.Client.ID)
 }
 return summary,errors.Join(errs...)
}

func IntervalFromEnv() time.Duration {
	if s := os.Getenv("POLL_INTERVAL"); s != "" {
		if d, e := time.ParseDuration(s); e == nil && d >= 5*time.Second {
			return d
		}
	}
	return 30 * time.Second
}
