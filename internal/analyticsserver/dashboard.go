package analyticsserver

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

type countRow struct {
	First  string
	Second string
	Count  int64
}
type outcomeRow struct {
	Command string
	Version string
	Success int64
	Failure int64
}
type durationRow struct {
	Command string
	P50     int64
	P95     int64
	Count   int64
}
type activityRow struct {
	Period string
	Count  int64
}
type retentionRow struct {
	Week      string
	Active    int64
	Returning int64
}
type installationRow struct {
	ID        string
	LastSeen  string
	Providers string
	Device    string
	Platform  string
	Commands  int64
}
type dashboard struct {
	Commands      []countRow
	Outcomes      []outcomeRow
	Durations     []durationRow
	Daily         []activityRow
	Weekly        []activityRow
	Retention     []retentionRow
	Providers     []activityRow
	Devices       []activityRow
	Installations []installationRow
}

// DashboardHandler is exclusively for a separate, tailnet-private loopback listener.
// Access control is enforced by the proxy, not by this handler.
func (s *Service) DashboardHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.URL.Path != "/oct-analytics" && r.URL.Path != "/oct-analytics/" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.URL.RawQuery == "confirm=clear" {
				if err := clearConfirmationTemplate.Execute(w, s.clearCSRF); err != nil {
					return
				}
				return
			}
			if r.URL.RawQuery != "" {
				http.Error(w, "invalid dashboard request", http.StatusBadRequest)
				return
			}
			data, err := s.statistics()
			if err != nil {
				http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
				return
			}
			if err := dashboardTemplate.Execute(w, data); err != nil {
				// An interrupted response cannot be replaced with a useful status.
				return
			}
		case http.MethodPost:
			if r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				http.Error(w, "invalid dashboard request", http.StatusBadRequest)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1024)
			if err := r.ParseForm(); err != nil || len(r.PostForm) != 2 ||
				len(r.PostForm["csrf"]) != 1 || len(r.PostForm["confirm"]) != 1 ||
				subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(s.clearCSRF)) != 1 ||
				r.PostForm.Get("confirm") != "DELETE ALL" {
				http.Error(w, "invalid confirmation", http.StatusForbidden)
				return
			}
			if err := s.clearRecords(); err != nil {
				http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
				return
			}
			http.Redirect(w, r, "/oct-analytics", http.StatusSeeOther)
		default:
			w.Header().Set("Allow", "GET, HEAD, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// Read rows to completion before starting another query: one SQLite connection
// serializes writes and avoids unbounded connections or SQLITE_BUSY storms.
func (s *Service) statistics() (dashboard, error) {
	var d dashboard
	rows, err := s.db.Query(`SELECT command, source, COUNT(*) FROM events WHERE event='oct_command_started' GROUP BY command, source ORDER BY command, source`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v countRow
		if err = rows.Scan(&v.First, &v.Second, &v.Count); err != nil {
			break
		}
		d.Commands = append(d.Commands, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`SELECT command, version, SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), SUM(CASE WHEN success=0 THEN 1 ELSE 0 END) FROM events WHERE event='oct_command_finished' GROUP BY command, version ORDER BY command, version`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v outcomeRow
		if err = rows.Scan(&v.Command, &v.Version, &v.Success, &v.Failure); err != nil {
			break
		}
		d.Outcomes = append(d.Outcomes, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`WITH ranked AS (
		SELECT command, duration_ms, ROW_NUMBER() OVER (PARTITION BY command ORDER BY duration_ms) AS rn,
		COUNT(*) OVER (PARTITION BY command) AS n FROM events WHERE event='oct_command_finished'
	) SELECT command, MAX(CASE WHEN rn=(n+1)/2 THEN duration_ms END),
	MAX(CASE WHEN rn=(n*95+99)/100 THEN duration_ms END), MAX(n)
	FROM ranked GROUP BY command ORDER BY command`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v durationRow
		if err = rows.Scan(&v.Command, &v.P50, &v.P95, &v.Count); err != nil {
			break
		}
		d.Durations = append(d.Durations, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`SELECT substr(timestamp,1,10), COUNT(DISTINCT distinct_id) FROM events WHERE event='oct_command_started' AND source IN ('cli','menubar') GROUP BY substr(timestamp,1,10) ORDER BY 1 DESC`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v activityRow
		if err = rows.Scan(&v.Period, &v.Count); err != nil {
			break
		}
		d.Daily = append(d.Daily, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`WITH weeks AS (
		SELECT date(timestamp, '-' || ((CAST(strftime('%w',timestamp) AS INTEGER)+6)%7) || ' days') AS week, distinct_id
		FROM events WHERE event='oct_command_started' AND source IN ('cli','menubar')
	) SELECT week, COUNT(DISTINCT distinct_id) FROM weeks GROUP BY week ORDER BY week DESC`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v activityRow
		if err = rows.Scan(&v.Period, &v.Count); err != nil {
			break
		}
		d.Weekly = append(d.Weekly, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}

	rows, err = s.db.Query(`WITH weeks AS (
		SELECT DISTINCT date(timestamp, '-' || ((CAST(strftime('%w',timestamp) AS INTEGER)+6)%7) || ' days') AS week, distinct_id
		FROM events WHERE event='oct_command_started' AND source IN ('cli','menubar')
	) SELECT current.week, COUNT(*), SUM(CASE WHEN previous.distinct_id IS NULL THEN 0 ELSE 1 END)
	FROM weeks AS current LEFT JOIN weeks AS previous
	ON previous.distinct_id=current.distinct_id AND previous.week=date(current.week,'-7 days')
	GROUP BY current.week ORDER BY current.week DESC`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v retentionRow
		if err = rows.Scan(&v.Week, &v.Active, &v.Returning); err != nil {
			break
		}
		d.Retention = append(d.Retention, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}
	rows, err = s.db.Query(`SELECT json_each.value, COUNT(*) FROM installations, json_each(installations.providers)
		GROUP BY json_each.value ORDER BY COUNT(*) DESC, json_each.value`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v activityRow
		if err = rows.Scan(&v.Period, &v.Count); err != nil {
			break
		}
		d.Providers = append(d.Providers, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}
	rows, err = s.db.Query(`SELECT device_family, COUNT(*) FROM installations GROUP BY device_family ORDER BY COUNT(*) DESC, device_family`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v activityRow
		if err = rows.Scan(&v.Period, &v.Count); err != nil {
			break
		}
		d.Devices = append(d.Devices, v)
	}
	if err = finishRows(rows, err); err != nil {
		return d, err
	}
	rows, err = s.db.Query(`SELECT i.distinct_id, i.last_seen, i.providers, i.device_family, i.os, i.arch,
		(SELECT COUNT(*) FROM events e WHERE e.distinct_id=i.distinct_id AND e.event='oct_command_started')
		FROM installations i ORDER BY i.last_seen DESC, i.distinct_id LIMIT 100`)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var v installationRow
		var selected, osName, arch string
		if err = rows.Scan(&v.ID, &v.LastSeen, &selected, &v.Device, &osName, &arch, &v.Commands); err != nil {
			break
		}
		var providers []string
		if err = json.Unmarshal([]byte(selected), &providers); err != nil {
			break
		}
		v.ID = v.ID[:8] + "…" + v.ID[len(v.ID)-4:]
		v.Providers = strings.Join(providers, ", ")
		if v.Providers == "" {
			v.Providers = "not specified"
		}
		v.Platform = osName + "/" + arch
		d.Installations = append(d.Installations, v)
	}
	return d, finishRows(rows, err)
}

func finishRows(rows *sql.Rows, scanError error) error {
	iterationError := rows.Err()
	closeError := rows.Close()
	if scanError != nil {
		return scanError
	}
	if iterationError != nil {
		return iterationError
	}
	return closeError
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>oct usage analytics</title><style>
body{font:16px system-ui,sans-serif;max-width:1100px;margin:2rem auto;padding:0 1rem;color:#18212b;background:#f8fafc}
h1,h2{margin-top:2rem}table{border-collapse:collapse;width:100%;margin-bottom:2rem;background:white}
th,td{padding:.55rem .7rem;border:1px solid #d5dce4;text-align:left}th{background:#e9eef4}
</style></head><body><h1>oct usage analytics</h1>
<form action="/oct-analytics" method="get"><button type="submit" name="confirm" value="clear">Clear usage records…</button></form>
<p>Observed installations, not people. Events and installation observations are retained for 90 days. Opt-outs and undelivered events are absent. Weekly returning means an installation with a started event in both this UTC Monday-start week and the previous week. Durations are nearest-rank percentiles. Provider counts show installations with an explicit latest selection, not AI requests or consumed tokens. Device families omit hostnames and serial numbers.</p>
<h2>Started commands by source</h2><table><tr><th>Command</th><th>Source</th><th>Started</th></tr>{{range .Commands}}<tr><td>{{.First}}</td><td>{{.Second}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Finished outcomes by version</h2><table><tr><th>Command</th><th>Version</th><th>Success</th><th>Failure</th></tr>{{range .Outcomes}}<tr><td>{{.Command}}</td><td>{{.Version}}</td><td>{{.Success}}</td><td>{{.Failure}}</td></tr>{{end}}</table>
<h2>Finished duration (ms)</h2><table><tr><th>Command</th><th>p50</th><th>p95</th><th>Finished</th></tr>{{range .Durations}}<tr><td>{{.Command}}</td><td>{{.P50}}</td><td>{{.P95}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Observed active installations — daily (UTC)</h2><table><tr><th>Date</th><th>Installations</th></tr>{{range .Daily}}<tr><td>{{.Period}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Observed active installations — weekly (UTC)</h2><table><tr><th>Week starting Monday</th><th>Installations</th></tr>{{range .Weekly}}<tr><td>{{.Period}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Observed weekly returning installations</h2><table><tr><th>Week starting Monday</th><th>Active</th><th>Returning from prior week</th></tr>{{range .Retention}}<tr><td>{{.Week}}</td><td>{{.Active}}</td><td>{{.Returning}}</td></tr>{{end}}</table>
<h2>Selected providers — installations</h2><table><tr><th>Provider</th><th>Installations</th></tr>{{range .Providers}}<tr><td>{{.Period}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Device families — installations</h2><table><tr><th>Device family</th><th>Installations</th></tr>{{range .Devices}}<tr><td>{{.Period}}</td><td>{{.Count}}</td></tr>{{end}}</table>
<h2>Installations (100 most recently seen)</h2><table><tr><th>Installation</th><th>Latest UTC</th><th>Selected providers</th><th>Device</th><th>OS/arch</th><th>Started commands</th></tr>{{range .Installations}}<tr><td>{{.ID}}</td><td>{{.LastSeen}}</td><td>{{.Providers}}</td><td>{{.Device}}</td><td>{{.Platform}}</td><td>{{.Commands}}</td></tr>{{end}}</table>
</body></html>`))

var clearConfirmationTemplate = template.Must(template.New("clear-confirmation").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Clear oct usage records</title><style>body{font:16px system-ui,sans-serif;max-width:760px;margin:2rem auto;padding:0 1rem}</style></head>
<body><h1>Clear all usage records?</h1>
<p>This permanently deletes stored command events and installation observations. It does not remove submitted feedback or installation IDs on users' devices; new runs can appear again.</p>
<form action="/oct-analytics" method="post"><input type="hidden" name="csrf" value="{{.}}">
<button type="submit" name="confirm" value="DELETE ALL">Delete usage records</button></form>
<p><a href="/oct-analytics">Cancel</a></p></body></html>`))
