package extraction

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	_ "embed"
)

//go:embed viewer.js
var viewerJS string

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SatisFacts Report</title>
<style>
:root {
  color-scheme: dark;
  --bg: #0e0e12;
  --surface: #1a1a24;
  --surface2: #22222e;
  --border: #33334a;
  --text: #e0e0e8;
  --text-dim: #8888a0;
  --accent: #00d4ff;
  --accent2: #ff6464;
  --green: #00ff80;
  --orange: #ffa500;
  --purple: #9370db;
  --yellow: #ffc800;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

html { background: var(--surface); }

body {
  background: var(--surface);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  line-height: 1.5;
  min-height: 100vh;
}

#loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  font-size: 1.2em;
  color: var(--accent);
}

#loading .spinner {
  display: inline-block;
  width: 24px;
  height: 24px;
  border: 3px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-right: 12px;
}

@keyframes spin { to { transform: rotate(360deg); } }

/* #app visibility controlled by JS inline style */

header {
  background: var(--bg);
  border-bottom: 2px solid var(--border);
  padding: 12px 20px;
  position: sticky;
  top: 0;
  z-index: 100;
}

header .header-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

header h1 {
  font-size: 1.3em;
  color: var(--accent);
  white-space: nowrap;
}

#search {
  flex: 1;
  min-width: 200px;
  max-width: 400px;
  padding: 8px 12px;
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 13px;
}

#search:focus {
  outline: none;
  border-color: var(--accent);
}

#search::placeholder { color: var(--text-dim); }

#expert-toggle {
  padding: 6px 12px;
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
  transition: all 0.15s;
}

#expert-toggle:hover {
  border-color: var(--accent);
  color: var(--text);
}

#expert-toggle.active {
  background: var(--accent);
  color: var(--bg);
  border-color: var(--accent);
  font-weight: 600;
}

.type-path-col { display: none; word-break: break-all; }
body.expert-mode .type-path-col { display: table-cell; }

#tabs {
  display: flex;
  gap: 4px;
  padding: 8px 20px;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
  overflow-x: auto;
  position: sticky;
  top: 52px;
  z-index: 99;
}

.tab-btn {
  padding: 8px 16px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
  transition: all 0.15s;
}

.tab-btn:hover {
  background: var(--surface2);
  color: var(--text);
}

.tab-btn.active {
  background: var(--accent);
  color: var(--bg);
  font-weight: 600;
}

.tab-content {
  display: none;
  padding: 20px;
  width: 100%;
  max-width: 2560px;
  margin: 0 auto;
}

.tab-content.active { display: block; }

.section {
  margin-bottom: 24px;
}

.section h2 {
  font-size: 1.1em;
  color: var(--accent);
  margin-bottom: 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 3px solid var(--accent);
  border-radius: 8px;
  padding: 12px 16px;
}

.card .label {
  font-size: 0.75em;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.card .value {
  font-size: 1.4em;
  font-weight: 700;
  color: var(--text);
  margin: 4px 0;
  word-break: break-word;
  overflow-wrap: break-word;
}

.card .desc {
  font-size: 0.8em;
  color: var(--text-dim);
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  table-layout: fixed;
}

thead th {
  background: var(--surface2);
  color: var(--text-dim);
  text-align: left;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  border-bottom: 2px solid var(--border);
  white-space: nowrap;
  position: sticky;
  top: 0;
}

thead th:hover { color: var(--accent); }

thead th.sort-asc::after { content: " \25B2"; color: var(--accent); }
thead th.sort-desc::after { content: " \25BC"; color: var(--accent); }

tbody td {
  padding: 6px 12px;
  border-bottom: 1px solid var(--border);
  overflow: hidden;
  text-overflow: ellipsis;
}

tbody tr:hover { background: var(--surface2); }

.scrollable {
  max-height: 600px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
}

.scrollable table thead th {
  position: sticky;
  top: 0;
  z-index: 1;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.8em;
  font-weight: 600;
}

.badge.green { background: var(--green); color: var(--bg); }
.badge.orange { background: var(--orange); color: var(--bg); }
.badge.red { background: var(--accent2); color: var(--bg); }
.badge.yellow { background: #ffc107; color: var(--bg); }
.badge.blue { background: #4fc3f7; color: var(--bg); }
.badge.gray { background: #666; color: var(--text); }

.bar-container {
  display: flex;
  align-items: center;
  position: relative;
  height: 20px;
  background: var(--surface2);
  border-radius: 4px;
  overflow: hidden;
  min-width: 80px;
}

.bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s;
}

.bar-label {
  position: absolute;
  right: 6px;
  font-size: 0.75em;
  color: var(--text);
  text-shadow: 0 0 4px rgba(0,0,0,0.8);
}

details {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 8px;
  padding: 8px 12px;
}

details summary {
  cursor: pointer;
  color: var(--text);
  font-weight: 600;
  padding: 4px 0;
}

details summary:hover { color: var(--accent); }

details[open] { padding-bottom: 12px; }

details > table, details > .scrollable {
  margin-top: 8px;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  border-bottom: 1px solid var(--border);
}

.stat-row .label { color: var(--text-dim); }
.stat-row .value { font-weight: 600; }

.stat-row.overclocked .value { color: var(--accent2); }
.stat-row.underclocked .value { color: #6495ED; }

.empty-state {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-dim);
  font-size: 1.1em;
}

.filter-bar input[type="text"],
.filter-bar select {
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 13px;
}

.filter-bar input[type="text"]:focus,
.filter-bar select:focus {
  outline: none;
  border-color: var(--accent);
}

.filter-bar input[type="text"]::placeholder { color: var(--text-dim); }

.filter-bar select option {
  background: var(--surface);
  color: var(--text);
}

.filter-bar button {
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s;
}

.filter-bar button:hover:not(:disabled) {
  background: var(--border);
}

.filter-bar button:disabled {
  opacity: 0.4;
  cursor: default;
}

@media (max-width: 768px) {
  .card-grid { grid-template-columns: 1fr 1fr; }
  .tab-content { padding: 12px; }
  #tabs { padding: 8px 12px; }
  header { padding: 8px 12px; }
  #search { max-width: none; }
}
</style>
</head>
<body>
<div id="loading"><span class="spinner"></span>Decompressing save data...</div>
<div id="app">
  <header>
    <div class="header-row">
      <h1 id="title">Save Report</h1>
      <input type="text" id="search" placeholder="Search in current tab...">
      <button id="expert-toggle" title="Show/hide full type paths">Expert Mode</button>
    </div>
  </header>
  <nav id="tabs"></nav>
  <div id="tab-overview" class="tab-content"></div>
  <div id="tab-structures" class="tab-content"></div>
  <div id="tab-buildings" class="tab-content"></div>
  <div id="tab-clockspeeds" class="tab-content"></div>
  <div id="tab-power" class="tab-content"></div>
  <div id="tab-production" class="tab-content"></div>
  <div id="tab-manufacturing" class="tab-content"></div>
  <div id="tab-storage" class="tab-content"></div>
  <div id="tab-transport" class="tab-content"></div>
  <div id="tab-map" class="tab-content"></div>
  <div id="tab-nuclear" class="tab-content"></div>
  <div id="tab-collectibles" class="tab-content"></div>
  <div id="tab-extras" class="tab-content"></div>
</div>
<footer style="text-align:center;padding:16px 20px;color:var(--text-dim);font-size:0.8em;border-top:1px solid var(--border);">
  Generated by SatisFacts — Built by Physox
</footer>
<script id="embedded-data" type="application/octet-stream">__EMBEDDED_DATA__</script>
<script>__VIEWER_JS__</script>
</body>
</html>`

// ReportData is the data structure embedded in the HTML report.
// It excludes the large "objects" array to keep the file size small.
type ReportData struct {
	Header          map[string]interface{} `json:"header"`
	Mode            string                 `json:"mode"`
	TotalObjects    int                    `json:"totalObjects"`
	TotalBuildings  int                    `json:"totalBuildings"`
	ObjectsWritten  int                    `json:"objectsWritten"`
	Buildings        map[string]int         `json:"buildings"`
	Blueprints       map[string]int         `json:"blueprints,omitempty"`
	Pets             interface{}            `json:"pets,omitempty"`
	Extraction      interface{}            `json:"extraction"`
	Structures      interface{}            `json:"structures"`
	Production      interface{}            `json:"production"`
	PowerGridData   interface{}            `json:"powerGridData"`
	GameProgression interface{}            `json:"gameProgression"`
	Analytics       interface{}            `json:"analytics"`
	Levels          interface{}            `json:"levels"`
	SignDisplayNames map[string]string     `json:"signDisplayNames,omitempty"`
}

// GenerateHTML creates a single-file interactive HTML report with embedded
// compressed JSON data. The data is gzip-compressed and base64-encoded.
func GenerateHTML(report ReportData, outputPath string) error {
	// Marshal report data to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshalling report data: %w", err)
	}

	// Gzip compress
	var gzipBuf bytes.Buffer
	gw := gzip.NewWriter(&gzipBuf)
	if _, err := gw.Write(jsonData); err != nil {
		return fmt.Errorf("gzip compressing: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(gzipBuf.Bytes())

	// Build HTML — use string replacement instead of fmt.Sprintf to avoid
	// CSS percent signs being interpreted as format verbs.
	html := strings.Replace(htmlTemplate, "__EMBEDDED_DATA__", encoded, 1)
	html = strings.Replace(html, "__VIEWER_JS__", viewerJS, 1)

	// Write to file
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("writing HTML file: %w", err)
	}

	return nil
}
