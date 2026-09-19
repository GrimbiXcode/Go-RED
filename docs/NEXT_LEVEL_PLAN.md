# Go-RED: Plan für das nächste Level

Stand: 2026-09-18, Branch `claude/projekt-analyse-verbesserung-znm6c6`, Basis `main` (0c19b3a).

Dieses Dokument ist das Ergebnis einer vollständigen Analyse des Projekts (Backend, Frontend,
Build/CI, Doku) inklusive eines echten Durchlaufs des ausgelieferten UI im Browser
(Go-Binary + `vite build`, Playwright-Screenshots, REST-Gegenproben). Alle Aussagen unter
"verifiziert" wurden im Code nachgelesen und/oder im laufenden System reproduziert.

---

## 0. Kurzfassung

**Das Backend hat Breite, das Produkt hat keine Tiefe.** 47 Node-Typen, ~25.000 Zeilen Go,
viele Tests. Aber die Kernschleife eines Flow-Editors, **Flow bauen → Deploy → Nachrichten live
sehen**, funktioniert im ausgelieferten UI nicht: Deploy ist für jeden über das UI angelegten
Flow ein No-op, das Debug-Panel bleibt für immer auf "Loading messages…", verschobene Nodes
springen zurück und werden nie gespeichert. Das sind keine Feinschliff-Themen, sondern drei
Blocker, die jede weitere UI-Arbeit entwerten.

**Das Frontend ist ein Prototyp (5.100 Zeilen), kein Editor.** Ein einziger `useState`-Monolith
statt Store, drei bis vier WebSocket-Verbindungen pro Browser-Tab, ein generisches
Schema-Formular statt Node-Editoren (Function-Code in einem einzeiligen Textfeld, Switch-Regeln
als JSON-Textarea), kein Undo, keine Shortcuts, kein Kontextmenü, kein Copy/Paste.

**Visuell fehlt eine eigene Identität.** Die Entscheidung "nur die drei Go-Markenfarben" hat
dazu geführt, dass acht Node-Kategorien in sechs Blautönen dargestellt werden und auf dem
Canvas nicht unterscheidbar sind. Dazu kommen fremde Material-Icons mit hartkodierten
Zufallsfarben auf den Nodes, 30-px-Nodes mit 12-px-Text, WCAG-Kontrastverstöße im Header und
auf hellen Nodes, JSON-Dumps im Info-Panel und ein gemischt deutsch/englisches UI.

Der Plan unten hat sieben Phasen. Phase 0 (Stabilisieren) und Phase 1 (Frontend-Fundament)
sind Voraussetzung für alles Weitere. Das visuelle Redesign (Phase 4) ist bewusst erst nach dem
Node-Editor-Modell (Phase 3) eingeplant, weil ein hübsches UI auf einem kaputten Datenmodell
nicht trägt.

---

## 1. Ist-Analyse

### 1.1 Verifizierte Bugs (Blocker zuerst)

| # | Befund | Wo | Wie verifiziert |
|---|---|---|---|
| B1 | **Deploy ist ein No-op** für jeden Flow, den das UI kennt. `CreateFlow` legt Flows sofort in `e.flows` (Status *inactive*) ab; der REST-Handler wertet "Flow in `e.flows`" als "schon deployed" und antwortet mit Erfolg, ohne `engine.Deploy` aufzurufen. Auch `engine.Deploy` selbst lehnt Flows ab, die in `e.flows` liegen. Die Engine vermischt "bekannt" und "aktiv" in einer Map. | `cmd/go-red/main.go:320-333`, `internal/engine/engine.go:475-478`, `engine.go:781-783` | Demo-Flow per REST angelegt, im UI "Deploy" geklickt → Server-Log "already deployed, returning success", `GET /api/flows` zeigt weiterhin `draft`, Inject-Intervall startet nie, `GET /api/messages` leer. |
| B2 | **WebSocket-Hub verklebt Nachrichten.** Der Writer hängt wartende Nachrichten mit `\n` getrennt in *denselben* Frame (Gorilla-Chat-Beispiel). Der Client macht `JSON.parse` auf den ganzen Frame → `SyntaxError: Unexpected non-whitespace character after JSON`. Folge: Antworten gehen verloren, das Debug-Panel bleibt auf "Loading messages…". | `cmd/go-red/websocket/hub.go:287-303`, `web/src/hooks/useWebSocket.ts:515` | Browser-Console beim Öffnen des Debug-Tabs; Panel zeigt dauerhaft "Loading messages…" trotz zweier beantworteter `message:log`-Requests im Server-Log. |
| B3 | **Node-Positionen gehen verloren.** `onNodeDragStop` schreibt nur in den ReactFlow-Lokalstate, nie in den Flow-State. "Speichern" sendet die alten Positionen; jeder Flow-Update (z. B. Deploy, Statusänderung) setzt alle Nodes per `useEffect` auf die alten Positionen zurück. | `web/src/components/FlowCanvas.tsx:132-141`, `:105-108` | Node um (150, 80) px gezogen, "Speichern" → REST liefert weiterhin `{x:300,y:120}`; nach "Deploy" springt der Node um (−150, −80) zurück. |
| B4 | **Mehrere WebSocket-Verbindungen pro Tab.** `useWebSocket()` öffnet bei jedem Aufruf eine eigene Verbindung; aufgerufen in `useFlows`, `useMessageLog`, `WebSocketStatus` und in *jedem* `InjectNode`. | `web/src/hooks/useWebSocket.ts:675-681`, `InjectNode.tsx:23`, `WebSocketStatus.tsx:9` | Playwright zählt 2 Verbindungen nach dem Laden, 3 nach Öffnen eines Flows mit einem Inject-Node, 4 nach Öffnen des Debug-Tabs. Server-Log bestätigt "Total clients: 4". |
| B5 | **Kein Echtzeit-Push.** Die Engine hat keinen Kanal zum Hub. `node:status` wird nur als Antwort auf eine Client-Anfrage gesendet, Debug-Nachrichten nur per `message:log`-Pull ("Refresh"-Button). Der Debug-Node schreibt nach `stderr` und in einen Puffer, den kein Client je sieht. | `internal/engine/engine.go` (kein Broadcast), `cmd/go-red/websocket/integration.go:122-129`, `internal/nodes/debug/node.go:47-65` | Code-Lesung; README verspricht "Real-time WebUI – Live updates via WebSocket". |
| B6 | **Status-Enum driftet.** Frontend setzt `'deployed'` und `'stopped'`, der generierte `FlowStatus`-Typ kennt nur `draft/running/error/deploying/undeploying`; Header-Logik prüft deshalb beide. | `web/src/hooks/useFlows.ts:138,161`, `web/src/types/generated.ts:1044`, `FlowEditor.tsx:245-249` | Code-Lesung; `tsc` meldet es nicht, weil per `as FlowStatus` gecastet wird. |
| B7 | **ESLint hat keine Konfiguration.** `npm run lint` bricht mit "couldn't find a configuration file" ab. CI schluckt das per `\|\| echo`. | `web/package.json:10`, `.github/workflows/web-tests.yml:36`, `web-ci.yml:37` | `npm run lint` lokal ausgeführt. |
| B8 | **Go-Code ist nicht gofmt-konform** (Spaces statt Tabs, Commit b888c56 hat das bewusst so gemacht). Der CI-Check gibt nur eine Warnung aus. | `.github/workflows/go.yml:44-51`, `gofmt -l .` listet praktisch alle Dateien | `gofmt -l .` lokal. |
| B9 | **20-MB-Binary zweimal committed** (`go-red`, `tmp/go-red`). Repo-Pack 10 MB, jede Änderung am Binary verdoppelt das. | Git-Index | `git ls-files`, `stat`. |
| B10 | **Switch-Node hat dynamische Outputs, die Metadaten aber statisch 2.** Regeln 3..n sind im Editor nicht verdrahtbar. Das Schema kennt kein "Anzahl Outputs hängt von Config ab". | `internal/nodes/switchnode/node.go:12-17, 338-341`, `internal/registry/registry.go:111-153` | Code-Lesung, Kommentar im Node bestätigt es selbst. |
| B11 | **Debug-Logging in Produktionscode**: `console.log('[FRONTEND] …')`, `console.log('[FlowCanvas] …')` bei jedem Render; Server loggt jede WS-Nachricht und Node-Config (`[DEBUG] [ENGINE] … config: %v`) inkl. potenzieller Secrets. | `useFlows.ts`, `FlowCanvas.tsx:100,107`, `engine.go:497-501` | Browser-Console (13 Einträge für einen Klickpfad). |
| B12 | **Dockerfile baut mit Go 1.21**, `go.mod` verlangt 1.25; `alpine:3.18` ist EOL. Image-Build schlägt fehl. | `Dockerfile:5`, `go.mod:3` | Code-Lesung. |
| B13 | **Kein SPA-Fallback**: `http.FileServer` ohne Index-Fallback; jede Route außer `/` liefert 404 nach Reload. | `cmd/go-red/main.go:160` | Code-Lesung. |
| B14 | **Data Race + Panic im Hub.** `BroadcastToClient` schließt `client.send` und löscht aus `h.clients` ohne Lock, aus beliebigen Goroutinen; `Run` mutiert die Map unter `RLock`. Kein `recover()` in den Pumps → ein Panic im Handler beendet den Prozess. | `cmd/go-red/websocket/hub.go:197-202, 146-157` | Audit mit Race-Detector: `DATA RACE` hub.go:200/201 vs. 125/135, `panic: send on closed channel`. |
| B15 | **`Undeploy` auf einen per UI angelegten Flow panict.** Der Stub aus `CreateFlow` hat `cancel == nil`; `Undeploy` ruft ihn ohne Prüfung. Über WS `flow:undeploy` erreichbar → Server-Absturz. | `internal/engine/engine.go:674, 781-788` | Audit, reproduziert. |
| B16 | **Message-IDs sind kaputt.** `submitMessage` überschreibt die UUID mit `"msg-" + string(rune(counter))` → nicht druckbare, kollidierende IDs. | `internal/engine/engine.go:442` | Code-Lesung. |
| B17 | **Shutdown panict.** `Stop` schließt `e.msgChan`, während Node-Goroutinen noch `submitMessage` aufrufen; `worker()` ruft `wg.Add` nach möglichem `wg.Wait`. | `engine.go:207, 221` | Audit, reproduziert. |
| B18 | **Flow-IDs ungeprüft in Dateipfaden.** `filepath.Join(base, "flows", id+".json")` mit ID direkt aus REST-Pfad/WS-Payload; kein Muster-Check. Path-Traversal über `flow:delete`/`flow:get`/`DELETE /api/flows/..%2F..` erreichbar. | `internal/state/manager.go:38, 49, 94` | Code-Lesung; Audit hat den Traversal-Pfad ausgeführt. |
| B19 | **Persistenz nicht atomar, Korruption stumm.** `os.WriteFile` ohne temp+rename/fsync; `LoadAllFlows` überspringt defekte Dateien ohne Log. Gespeichert wird die *interne* Engine-Struktur (Dauern in Nanosekunden, Status `active`), nicht das Wire-Format; `version` wird nie gelesen. Beim Beenden wird `Status: active` gespeichert und beim Start alles blind deployed. | `manager.go:33-39, 82-84`, `main.go:183-186` | Audit, reproduziert (defekte Datei → 1 Flow, kein Fehler). |
| B20 | **`web/node_modules/flatted/golang/...` wird als Go-Paket mitgebaut**, weil es im Modulbaum liegt und kein eigenes `go.mod` hat. | `go list ./...` | `go list ./... \| grep flatted` → 1 Treffer. |

### 1.2 Technische Schwächen (Architektur)

**Frontend (`web/`)**

- **State-Management fehlt.** `useFlows` hält alles in einem `useState`-Objekt und reicht es per
  Context durch. Jede Statusänderung rendert den gesamten Editor neu. `zustand` ist installiert,
  wird aber nicht benutzt; die 1.100-Zeilen-`AGENTS.md`-Dateien beschreiben eine Zustand-Architektur,
  die nie gebaut wurde (sie sagen das inzwischen selbst).
- **Zwei Schreibpfade ohne Quelle der Wahrheit.** Canvas-Änderungen gehen als `node:add/update`,
  `connection:add` sofort per WebSocket an den Server; "Speichern" schickt zusätzlich den ganzen
  Flow per `PUT`. Es gibt kein Dirty-Tracking gegen den Server-Stand, keine Konfliktbehandlung,
  keinen Undo-Stack.
- **Flaches Property-Schema.** `registry.Property` kennt nur `type/enum/min/max/pattern`. Damit
  lässt sich kein Node-RED-artiger Editor generieren: kein TypedInput (`msg.` / `flow.` / `global.`
  / `str` / `num` / `json` / `env`), keine Listen (Switch-Regeln, Change-Regeln, HTTP-Header),
  kein Code-Feld mit Sprache, keine Credentials, keine dynamische Output-Anzahl, keine
  Abhängigkeiten zwischen Feldern (z. B. `rateLimit` nur bei `mode = rate`). Ergebnis heute:
  Function-Code in einem einzeiligen `<input>`, Switch-Regeln als JSON-Textarea.
- **ReactFlow 11** ist eingefroren; aktuell ist `@xyflow/react` 12 (Dark-Mode-Support, bessere
  Performance, aktive Wartung). Default-Controls inkl. "React Flow"-Attribution sind sichtbar.
- **Editor-Grundfunktionen fehlen**: Undo/Redo, Copy/Paste/Duplicate, Multi-Select-Aktionen,
  Snap-to-Grid, Ausrichten, Tastaturkürzel, Kontextmenü, Node-Suche, Flow-Umbenennen,
  Flow-Tabs sortieren, Wire-Splice (Node auf Kante ziehen), Doppelklick-Quick-Add.
- **Tests sind Form-, keine Verhaltenstests.** 41 Tests, davon 23 prüfen die Form von TypeScript-
  Objekten; kein Test rendert den Editor und interagiert. Kein E2E.
- **Kein i18n**: Texte hart kodiert, gemischt DE/EN ("Speichern", "Search nodes…", "Loading flows…",
  "Flow löschen", "Configure").

**Backend (`cmd/`, `internal/`)**

- **Flow-Lifecycle-Modell.** `e.flows` hält sowohl gespeicherte als auch laufende Flows; "aktiv"
  wird nur über ein Status-Feld unterschieden. Daraus folgen B1 und ein unklarer Redeploy-Pfad
  (REST und WebSocket deployen unterschiedlich).
- **Kein Ereigniskanal Engine → Clients.** Node-Status, Debug-Ausgaben, Flow-Status,
  Fehler: nichts wird gepusht. Ein `EventBus` existiert (`internal/registry/eventbus.go`), wird
  aber nur flow-intern benutzt.
- **Logging**: `log.Printf` mit Präfixen wie `[DEBUG] [ENGINE]`, doppelte Zeilen, kein Level, kein
  strukturiertes Format; Node-Configs werden im Klartext geloggt.
- **Konfiguration** nur über Flags; keine Env-Variablen, keine Datei, kein `-version`.
- **Sicherheit**: keine Authentifizierung für REST und WebSocket, kein CORS-Konzept, `exec`-Node
  per Env-Flag, `http request` ohne SSRF-Schutz (dokumentiert), File-Nodes mit Pfad-Konfiguration.
  Für einen lokalen Dev-Server tolerierbar, für "Cloud/Kubernetes" (README) nicht.
- **Hygiene**: gofmt-Verstoß (B8), Binaries im Repo (B9), Dockerfile-Drift (B12),
  `tmp/`-Verzeichnis versioniert, Makefile-`test` mit `-race`, CI ohne `-race`.
- **Performance-Claims** ("> 100.000 msg/s", Worker-Pools, Batching, Object-Pooling in
  `docs/ARCHITECTURE.md`) sind nicht durch Benchmarks belegt; es gibt keine `Benchmark*`-Funktion
  im Repo. Der tatsächliche Pfad (siehe Anhang A) loggt pro Nachricht per `log.Printf`, klont
  pro Hop Maps, startet pro Hop zwei Goroutinen und hält eine globale Mutex für das Message-Log;
  der "Worker-Pool" aus 100 Goroutinen begrenzt nichts, weil jeder Worker pro Nachricht wieder
  eine Goroutine startet.
- **Geteilter, ungeschützter Zustand.** REST/WS-Handler mutieren `*engine.Flow` (AddNode,
  RemoveNode, `node.Config = …`) direkt, während Worker-Goroutinen dieselben Maps lesen und
  `SaveFlow` sie serialisiert. Zwei Clients oder "Editieren während Nachrichten fließen" ist ein
  `concurrent map`-Fatal.
- **Ungenutzte Konfiguration.** `EngineConfig.MaxRetries/RetryBackoff`,
  `FlowConfig.RetryPolicy/MaxConcurrency/Timeout` und die Flags `-max-workers`, `-max-messages`,
  `-plugin-dir` werden geparst, aber nie gelesen.
- **Function-Node ohne Abbruch.** goja-VM ohne `Interrupt`; `while(true)` im Nutzer-JS blockiert
  eine Goroutine dauerhaft.
- **Duplikate statt Node-SDK.** `cloneMap` in 21 Paketen, `floatPtr` in 18, TypedValue-Parser
  in 6; jeder `SetConfig` dekodiert Maps von Hand; `Execute(ctx interface{}, …)` mit 19
  ungeprüften `ctx.(context.Context)`-Casts.
- **Tests**: 450 Testfunktionen, alle grün, 0 Benchmarks. Der schwächste Punkt: `main_test.go`
  kopiert `handleDeployFlow` in den Test und **testet das Bug-Verhalten von B1 als erwartet**
  (`main_test.go:85-100, 170-197`). `HandleMessage` (WS-Integration) hat keinen einzigen Test.

Details mit Zeilenangaben in Anhang A.

### 1.3 Visuelle Bewertung

Grundlage: Screenshots des gebauten UI bei 1440×900 und 1024×700 mit einem Demo-Flow aus
neun Nodes verschiedener Kategorien (Inject, Function, Switch, 2× Debug, MQTT in, JSON,
HTTP request, Comment).

1. **Keine Identität.** Header ist ein Vollflächen-Balken in `#00ADD8` mit dem Text "Go-RED".
   Kein Logo, keine Wortmarke, kein Gopher. Die Farbe ist als Fläche zu grell; als Akzent wäre
   sie stark.
2. **Kontrast.** Weißer Text auf `#00ADD8` hat ein Kontrastverhältnis von ca. **2,6 : 1**
   (WCAG AA verlangt 4,5 : 1). Auf den helleren Node-Tönen (`gr-skyblue-300`, z. B. der
   JSON-Node) ist weißer 12-px-Text praktisch unlesbar (< 2 : 1).
3. **Kategorien sind nicht unterscheidbar.** Input, Output, Function, Network, Protocol, Parser
   und Dashboard sind sechs Blautöne derselben Hue. Auf dem Canvas sieht ein MQTT-Node aus wie
   ein Inject-Node. Flow-Control und Comment sind Fuchsia, was semantisch "Fehler/Löschen"
   signalisiert. Die Wurzel ist die Design-Entscheidung "nur drei Go-Farben" in
   `docs/FRONTEND_NODE_RED_REDESIGN.md`; sie ist gut gemeint, aber für ein Kategorien-System
   ungeeignet.
4. **Icons sind Fremdkörper.** Das Backend liefert Material-Design-SVGs mit hartkodierten
   Füllfarben (`fill="#2196F3"`, grüner Haken auf Inject, orange-roter Kreis auf Debug). Diese
   landen unverändert auf den blauen Nodes und in der Palette. Palette-Kategorien nutzen ein
   anderes (monochromes) Icon-Set → zwei Bildsprachen nebeneinander.
5. **Node-Geometrie.** 30 px hoch, 100 px min-Breite, 12-px-Text, 10-px-Quadratports mit weißem
   Rand. Wirkt nicht "kompakt wie Node-RED", sondern gedrängt. Kein Statustext unter dem Node,
   kein Icon-Well, keine visuelle Trennung Icon/Label. Inject-Button ist ein weißer Kreis mit
   `▶` als Textzeichen.
6. **Kanten.** 1,5-px-Grau ohne Hover-Zone, keine Markierung des Ports beim Verbinden, keine
   Animation für aktive Nachrichten.
7. **Panels.** Info-Tab zeigt rohe IDs, "Position X: 300.0" und JSON-Dumps in `<pre>`. Debug-Tab
   zeigt "Loading messages…" (B2) bzw. Karten mit "Message ID: …" und `flowId`-Chips statt eines
   kompakten Feeds (Zeit · Node-Name · `msg.payload : Object` mit klappbarem Baum).
8. **Palette.** Nur "Input" ist aufgeklappt, alle anderen Kategorien zu; Kategoriezeilen als
   pastellige Vollflächen-Buttons; Zähler in Grau ohne Bezug. Kein Hover-Vorschau-Node.
9. **Typografie.** Systemschrift, 13 px Basis, keine Hierarchie zwischen Panel-Titel,
   Label und Wert. Kein Mono-Font für Code/JSON außer Browser-Default.
10. **Zustände.** Kein Empty-State ("Select a flow to edit" in Grau), keine Skeletons, kein
    Onboarding, keine Fehlerzustände außer roter Text. Toasts sind Standard-Weiß.
11. **Dark Mode** fehlt vollständig, obwohl das Tailwind-Config eine `dark`-Skala anlegt.
12. **Responsiv.** Bei 1024 px Breite bleiben Palette (256 px) und Sidebar (320 px) fix, der
    Canvas schrumpft auf ~400 px.

---

## 2. Zielbild

Ein Flow-Editor, den man in fünf Minuten versteht und dem man in fünf Sekunden ansieht, dass er
ein eigenes Produkt ist: **Flow bauen → Deploy → Live-Debug**, ruhig, dicht, präzise.

Design-Prinzipien, die alle folgenden Entscheidungen leiten:

1. **Go ist Akzent, nicht Fläche.** Gopher-Blau `#00ADD8` für Fokus, Selektion, primäre Aktion,
   Wortmarke. Das UI-Chrome selbst ist neutral (helles Grau / dunkles Slate), damit die Nodes die
   Farbe tragen.
2. **Jede Kategorie hat eine eigene Hue.** Bewusster Bruch mit "nur drei Go-Farben". Die
   Go-Farben bleiben Marke; die Kategorie-Palette ist ein eigenes, abgestimmtes System mit
   geprüften Kontrasten (Text immer ≥ 4,5 : 1 auf der Node-Fläche).
3. **Live statt Refresh.** Alles, was die Engine weiß (Status, Debug, Fehler, Durchsatz), sieht der
   Nutzer ohne Klick innerhalb von 100 ms.
4. **Dichte mit Hierarchie.** Eine Typo-Skala (11/12/13/14/16), ein UI-Font (Inter), ein
   Mono-Font (JetBrains Mono), 4-px-Raster. Kompakt, aber lesbar.
5. **Eigene Editor-Widgets statt generischer Formulare.** TypedInput, Regel-Listen,
   Code-Editor, Credentials: die Node-Konfiguration ist der Kern des Produkts.
6. **Zugänglich und tastaturbedienbar.** WCAG AA, Fokus-Ringe, Shortcuts für alles Häufige.
7. **Eine Sprache pro Oberfläche.** Englisch als Default, Deutsch als Übersetzung, beides aus
   einer i18n-Datei.

---

## 3. Maßnahmenplan

Jede Phase ist ein Bündel review-barer PRs. Reihenfolge ist Abhängigkeitsreihenfolge, nicht
Wunschreihenfolge. Aufwände sind Schätzungen für eine Person in Vollzeit.

### Phase 0 — Stabilisieren (ca. 1 Woche) ✅ Voraussetzung für alles

| Maßnahme | Bezug |
|---|---|
| Engine: `e.flows` in `known` (gespeichert) und `active` (laufend) trennen; `Deploy` = `Undeploy`+`Deploy` atomar, wenn aktiv; REST- und WS-Deploy nutzen dieselbe Funktion; Flow-Status wird von der Engine gesetzt, nicht vom Client geraten; `Undeploy` ohne nil-Panic; `main_test.go` so ändern, dass er das *richtige* Verhalten testet | B1, B6, B15 |
| Hub: ein JSON-Dokument pro WebSocket-Frame (Batching-Schleife entfernen); Client-Map nur in `Run` mutieren; `recover()` in Read/Write-Pumps | B2, B14 |
| Engine: `Stop` schließt `msgChan` erst nach `wg.Wait`; Message-ID-Zähler als `strconv`/UUID; ID-Muster-Validierung vor jedem Dateizugriff | B16, B17, B18 |
| Persistenz: temp+rename+fsync; defekte Dateien loggen statt überspringen | B19 |
| `web/node_modules` aus dem Go-Modulbaum ausschließen (leeres `web/go.mod` oder Verzeichnis umbenennen) | B20 |
| Canvas: `onNodesChange` (Position) in den Flow-State spiegeln; Sync-Effekt nur bei Struktur-Änderung, nicht bei jedem Status-Update | B3 |
| Ein WebSocket-Client pro Tab (Modul-Singleton bzw. Store, `useWebSocket` liest nur) | B4 |
| `FlowStatus` bereinigen: `draft / deploying / running / stopping / stopped / error`; Frontend nutzt nur generierte Typen | B6 |
| ESLint-Config (flat config, `typescript-eslint`, `react-hooks`, `no-console`) anlegen; `npm run lint` blockierend in CI | B7, B11 |
| `gofmt -w .` in einem Commit; CI-Check blockierend; `golangci-lint` mit `govet, staticcheck, errcheck, gosec` | B8 |
| Binaries aus dem Index entfernen (`git rm --cached go-red tmp/go-red`), `.gitignore` ergänzen, Historie optional per `git filter-repo` verschlanken | B9 |
| `console.log` entfernen, Server-Logging auf `log/slog` mit Leveln umstellen, Node-Configs nicht loggen | B11 |
| Dockerfile auf `golang:1.25-alpine` + `alpine:3.20`, Frontend-Build im Multi-Stage | B12 |
| SPA-Fallback im FileServer | B13 |
| CI: `go test -race ./...`, `npm test`, `npm run lint`, `npm run build`, `tsc` alle blockierend; ein Workflow statt vier überlappender | B7, B8 |

Abnahme: die Repro aus dieser Analyse (Flow anlegen, Node ziehen, speichern, deployen, injizieren)
funktioniert im Browser; Browser-Console leer; genau eine WebSocket-Verbindung; CI rot bei
Lint/Format/Test-Fehlern.

**Status: umgesetzt** (Branch `claude/projekt-analyse-verbesserung-znm6c6`). Alle Punkte der
Tabelle sind erledigt; die Abnahme wurde mit einem Playwright-Skript gegen das gebaute UI
gefahren (14/14 Prüfungen: SPA-Fallback, Position nach Drag persistiert, Deploy setzt `running`,
kein Zurückspringen, Nachrichten fließen, Debug-Panel zeigt sie, Stop/Redeploy, Reload, genau
eine WebSocket-Verbindung, leere Console, Health-Endpoint). `go test -race ./...`, `npm run lint`,
`tsc`, Vitest und `vite build` sind grün.

Über den Plan hinaus gefunden und behoben:

- **Inject-Intervall hat nie gesendet.** Der Ticker aktualisierte nur ein Feld und startete erst,
  wenn der Node von außen ausgeführt wurde. Inject ist jetzt ein `EmittingNode` (Intervall und
  "einmal beim Deploy").
- **Deploy-Fehler waren unsichtbar.** Ein Node mit ungültiger Konfiguration (z. B. `mqtt in`
  ohne Broker) ließ den Deploy mit dem alten No-op scheinbar gelingen. Jetzt antwortet der Server
  mit 422 und einer lesbaren Meldung, der Flow bekommt Status `error`, das UI zeigt einen Toast.
- **Undeployte Flows verschwanden** aus der Liste bis zum Neustart; **importierte Flows** waren
  bis zum Neustart unsichtbar. Beides geht jetzt durch den Engine-Bestand.
- **Verschachtelter `ReactFlowProvider`** trennte `screenToFlowPosition` vom echten Viewport
  (Drop-Position bei Zoom/Pan falsch).
- Beim Beenden bleibt der persistierte Status erhalten; beim Start werden nur die Flows
  deployed, die beim letzten Lauf liefen.

Bewusst nicht in Phase 0: `FlowStatus` behält die fünf vorhandenen Werte
(`draft/running/error/deploying/undeploying`), weil Frontend und Backend jetzt dieselbe
generierte Quelle nutzen und ein zusätzliches `stopped` keinen Mehrwert hätte. `CheckOrigin`
und Auth bleiben Phase 6. Der WebSocket-Schreibpfad pro Node (`node:add` …) bleibt bis Phase 1
bestehen, läuft aber jetzt über `UpdateFlow` unter Lock.

### Phase 1 — Frontend-Fundament (1–2 Wochen)

- **Store** mit Zustand (bereits installiert): Slices `flows`, `editor` (Selection, Dirty, Undo-Stack,
  Viewport), `runtime` (Node-Status, Debug-Feed, Verbindungsstatus), `nodeTypes`. Selektive
  Subscriptions statt Context-Rerenders.
- **Ein Schreibpfad**: Canvas-Änderungen sind lokal (Undo-fähig) bis "Deploy"; Deploy = `PUT` des
  ganzen Flows + Deploy-Aufruf (wie Node-RED). WebSocket nur für Ereignisse vom Server und für
  Aktionen wie "Inject". Die per-Node-WS-Nachrichten (`node:add` …) entfallen.
- **Migration auf `@xyflow/react` 12** (Dark-Mode-Props, `nodeOrigin`, bessere Handles,
  `useNodesData`).
- **Testbasis**: Vitest + Testing-Library-Tests für Store und Editor-Interaktionen (Node ziehen,
  verbinden, löschen, Undo); Playwright-E2E-Smoke (die Repro aus Phase 0) in CI.
- **i18n**: `i18next` mit `en.json`/`de.json`; alle Strings raus aus den Komponenten.
- **Routing**: `/flow/:id` als URL, Reload landet im selben Flow.

**Status: umgesetzt.** Abweichung vom ursprünglichen Text: Änderungen bleiben nicht "lokal bis
Deploy", sondern werden als **Entwurf automatisch gespeichert** (debounced `PUT` des ganzen
Flows, geflusht vor Deploy, beim Flow-Wechsel und beim Verlassen der Seite). Das passt zum
Engine-Modell aus Phase 0 (Definition = Entwurf, laufende Instanz = Snapshot beim Deploy) und
verliert nichts bei einem Reload. "Dirty" heißt jetzt "Entwurf weicht vom laufenden Stand ab"
(`updatedAt > deployedAt`, neues Feld im Wire-Format). Der WebSocket transportiert keine
Mutationen mehr; die REST-Handler benachrichtigen den Hub, der `flow:status`/`flow:list` pusht.
Umgesetzt außerdem: Zustand-Stores (`flowStore` mit Undo/Redo, `editorStore`, `runtimeStore`,
`notificationStore`), `@xyflow/react` 12, i18n (EN/DE, Umschalter im Menü), `/flow/:id`,
Ctrl+Z/Y, Vitest-Tests für Store, WebSocket-Client und Komponenten, Playwright-E2E gegen den
echten Go-Server als CI-Job. Details in `docs/ARCHITECTURE.md`, Abschnitt "Frontend
Architecture".

### Phase 2 — Echtzeit-Protokoll (ca. 1 Woche, Backend + Frontend)

- **Engine-Events**: ein prozessweiter `EventBus`-Kanal (`registry/eventbus.go` verallgemeinern),
  auf den die Engine `flow.status`, `node.status`, `node.error`, `debug.message`,
  `flow.metrics` (msg/s pro Node) publiziert. Der Hub abonniert und pusht an Clients; Clients
  abonnieren pro Flow (`subscribe {flowId}`), damit nicht jeder alles bekommt.
- **Debug-Node** publiziert statt nach `stderr` zu schreiben: `{nodeId, nodeName, topic,
  payload, timestamp, level}`; Ringpuffer pro Flow im Server für Nachladen beim Verbinden.
- **Node-Status-API** für Nodes (`ctx.SetStatus(fill, shape, text)` wie Node-RED), damit MQTT,
  HTTP, Delay etc. ihren Zustand unter dem Node zeigen.
- **Protokoll-Datei** `docs/PROTOCOL.md` als einzige Quelle: alle Nachrichtentypen, Richtung,
  Payload; `cmd/gentypes` erweitert auf Event-Payloads.

### Phase 3 — Node-Editor-Modell (ca. 2 Wochen, Backend + Frontend)

- **Schema v2** in `internal/registry`:
  - `Property` bekommt `widget` (`text | textarea | number | boolean | select | typedInput |
    code | list | keyValue | credential | duration | json`), `typedInput.types`
    (`msg,flow,global,str,num,bool,json,env`), `list.item` (Sub-Schema), `language` für `code`,
    `visibleWhen` (Feldabhängigkeit), `group` (Tabs/Abschnitte), `label`, `help`.
  - `NodeMetadata` bekommt `outputs: {fixed: n}` **oder** `{fromProperty: "rules", labels: …}`,
    `color` (optional, überschreibt Kategorie), `help` (Markdown), `defaultName`.
  - Migration: alle 47 Nodes auf Schema v2 heben (der Großteil ist mechanisch; Switch, Change,
    Function, HTTP request, MQTT, Delay, Join/Split, Template brauchen echte Editoren).
- **Edit-Tray v2** im Frontend: Widget-Registry, die Schema v2 rendert; Name-Feld immer oben;
  Tabs "Properties / Description / Appearance" wie Node-RED; Validierung aus Schema
  (`required`, `pattern`, `min/max`) mit Inline-Fehlern; "Fertig" nur bei gültigem Formular.
- **Widgets**: TypedInput (Dropdown + Feld), RuleList (Drag-Sortierung, Operator-Auswahl),
  KeyValue-Liste, Code-Editor mit **CodeMirror 6** (JS-Syntax, Autocomplete für `msg.`),
  JSON-Editor mit Validierung, Duration-Feld.
- **Dynamische Ports**: Canvas rendert Outputs aus `outputs.fromProperty` und benennt sie mit
  Rule-Labels; Kanten auf entfernte Ports werden beim Speichern entfernt (mit Hinweis).

### Phase 4 — Visuelles Redesign (ca. 2 Wochen)

Design-Tokens v2 (Vorschlag, in Phase 4 gegen echte Screens abzustimmen):

| Token | Light | Dark | Verwendung |
|---|---|---|---|
| `--bg-app` | `#f5f6f8` | `#111318` | App-Hintergrund |
| `--bg-panel` | `#ffffff` | `#1a1d24` | Palette, Sidebar, Tray |
| `--bg-header` | `#1c2230` | `#0c0f14` | Kopfleiste (dunkel, ruhig) |
| `--bg-canvas` | `#fbfbfc` | `#14171d` | Canvas |
| `--grid-dot` | `#d6dbe3` | `#2a2f3a` | Punktraster |
| `--fg` / `--fg-muted` | `#1d2433` / `#6b7280` | `#e6e8ec` / `#9aa3b2` | Text |
| `--accent` | `#00ADD8` | `#33c3e6` | Gopher-Blau: Fokus, Selektion, Primär-Button, Wortmarke |
| `--danger` | `#CE3262` | `#e05a85` | Fuchsia: Fehler, Löschen |
| `--wire` / `--wire-hover` | `#9aa3b2` / `#00ADD8` | `#4b5563` / `#33c3e6` | Kanten |

Kategorie-Palette (Node-Fläche, jeweils mit dunklerer Kante und weißem Text ≥ 4,5 : 1; Werte sind
Startpunkt, in Phase 4 mit einem Kontrast-Validator prüfen):

| Kategorie | Fläche | Begründung |
|---|---|---|
| input | `#0E8FB0` (dunkleres Gopher-Blau) | Marke bleibt beim wichtigsten Einstieg |
| output | `#2F855A` (Grün) | "Ergebnis/OK" |
| function | `#5B4FCF` (Indigo) | Logik |
| flow-control | `#B7791F` (Amber) | Verzweigung/Zeit, statt Fuchsia (= Fehler) |
| network | `#7C3AED` (Violett) | |
| protocol | `#0F766E` (Teal) | verwandt mit network, aber unterscheidbar |
| parser | `#C05621` (Orange) | Transformation |
| storage | `#4A5568` (Slate) | |
| config | `#6B7280` (Grau, gestrichelter Rahmen) | wie Node-RED-Config-Nodes |
| comment | `#FEF3C7` mit dunklem Text | Notizzettel-Optik, klar kein Verarbeitungs-Node |

Weitere Punkte der Phase:

- **Icons**: Backend liefert nur einen **Icon-Namen** (`icon: "timer"`), Frontend rendert aus
  **Lucide** (MIT, monochrom, currentColor). Kein SVG-Markup mehr über die Leitung, kein DOMPurify
  nötig. Eine Bildsprache für Palette, Canvas, Tray.
- **Node-Geometrie**: 36 px hoch, Radius 6, Icon-Well links (28 px, 12 % dunkler als Fläche),
  Label 13 px/500, Statuszeile 11 px darunter (Punkt + Text), Ports 8 px rund mit 2-px-Rand in
  Flächenfarbe, Selektion = 2-px-Akzent-Outline + weicher Schatten, Hover = Aufhellung 6 %.
  Disabled = 50 % Opazität + Diagonalstreifen.
- **Kanten**: 2 px, 12-px-unsichtbare Hover-Zone, Hover/Selektion in Akzent, kurzer Puls
  entlang der Kante bei Nachricht (aus `flow.metrics`).
- **Header**: dunkle Leiste, Wortmarke "Go-RED" mit kleinem Gopher-Mark, rechts Deploy als
  Split-Button (Full / Modified) in Akzent, Verbindungsstatus als Punkt im Header (Statusleiste
  unten entfällt oder wird optional).
- **Palette**: Kategorien mit kleinem Farbchip (= Node-Fläche), standardmäßig alle offen,
  Zustand persistiert; Suchfeld mit `/`-Shortcut; Node-Zeile als Mini-Node (Icon-Well + Label).
- **Info-Tab**: Node-Name, Typ als Chip, Hilfe-Text (Markdown aus `NodeMetadata.help`),
  Eigenschaften als Key/Value-Tabelle statt JSON-Dump; Flow-Tab mit editierbarem Namen und
  Beschreibung.
- **Debug-Tab**: Feed wie Node-RED: Zeit · Node-Name · `msg.payload : Object`, klappbarer
  JSON-Baum (`react-json-tree`-artig, eigen), Filter nach Node, "nur aktueller Flow", Pause,
  Clear; Fehler in Danger-Farbe.
- **Empty-States**: kein Flow → Illustration + "Neuen Flow anlegen" + "Beispiel importieren";
  leerer Canvas → dezenter Hinweis "Node aus der Palette ziehen oder Doppelklick".
- **Dark Mode**: alle Tokens dual, Umschalter im Menü, `prefers-color-scheme` als Default.
- **Typografie**: Inter (variable, self-hosted) + JetBrains Mono; Skala 11/12/13/14/16/20.
- **Style-Guide-Seite** ausbauen zum Living-Styleguide (Tokens, Nodes aller Kategorien, Widgets,
  Debug-Einträge, Light/Dark nebeneinander).

### Phase 5 — Editor-Ergonomie (1–2 Wochen)

- Undo/Redo (Store-Historie), Copy/Paste/Duplicate (auch zwischen Flows), Multi-Select mit
  Rahmen, Verschieben per Pfeiltasten, Delete/Backspace.
- Snap-to-Grid (20 px), Ausrichten/Verteilen, Auto-Layout (dagre/elk) als Aktion.
- Kontextmenü (Rechtsklick auf Node/Kante/Canvas), Doppelklick auf Canvas = Quick-Add mit Suche,
  Node auf Kante ziehen = einfügen.
- Shortcuts: `Ctrl+S` Deploy, `Ctrl+Z/Y`, `Ctrl+C/V/D`, `Ctrl+F` Node-Suche, `Ctrl+E` Export,
  `Space+Drag` Pan, `+/-` Zoom, `?` Shortcut-Hilfe.
- Flow-Tabs: Umbenennen per Doppelklick, Drag-Sortierung, Kontextmenü (Duplizieren, Deaktivieren,
  Löschen), Dirty-Punkt am Tab.
- Import/Export im **Node-RED-JSON-Format** (Array aus Nodes mit `wires`), damit bestehende
  Flows aus Node-RED übernommen werden können; das ist ein echter Adoptions-Hebel.

### Phase 6 — Backend-Reife (parallel zu 3–5, ca. 2 Wochen)

- Persistenz: atomares Schreiben (temp + rename), `schemaVersion` im Flow-JSON, Migrationsfunktion,
  Backup-Rotation; optional SQLite als zweites Backend hinter dem `StateManager`-Interface.
- Auth: Admin-Token/Basic-Auth für REST und WS (konfigurierbar), CORS explizit, Rate-Limit auf
  Import/Deploy.
- Konfiguration: Flags + Env + optional `go-red.yaml`; `-version`; `/api/health`, `/api/version`,
  `/metrics` (Prometheus) mit Node-Durchsatz.
- Benchmarks (`go test -bench`) für Inject→Function→Debug, Switch-Fanout, Join; Ergebnisse in
  `docs/PERFORMANCE.md`; README-Claims entsprechend anpassen.
- Concurrency-Härtung nach den Befunden aus Anhang A (Race-Detector in CI, Goroutine-Leak-Tests
  bei Undeploy/Redeploy, Kontext-Abbruch in allen Netzwerk-Nodes).
- Node-Basisklasse (`nodes/base`) gegen Duplikate (Config-Dekodierung, Status, Logging) in 47
  Paketen.

### Phase 7 — Auslieferung & Doku (ca. 1 Woche)

- Frontend per `go:embed` ins Binary (ein Artefakt, kein `-web-dir` mehr), GoReleaser für
  Linux/macOS/Windows + Docker-Image (GHCR), Release-Workflow mit Changelog.
- README neu: Screenshots (Light/Dark), 60-Sekunden-Quickstart, Feature-Matrix vs. Node-RED.
- `AGENTS.md`-Dateien auf die Wahrheit kürzen (heute > 1.000 Zeilen pro Datei, teils
  Beschreibungen nie gebauter Architektur); `docs/ARCHITECTURE.md` mit echten Diagrammen
  (Engine-Lifecycle, Event-Fluss, Schema v2).

---

## 4. Reihenfolge, Aufwand, Abhängigkeiten

| Phase | Inhalt | Aufwand | Hängt ab von |
|---|---|---|---|
| 0 | Stabilisieren, CI hart — **erledigt** | 1 Woche | – |
| 1 | Store, ein Schreibpfad, xyflow 12, Tests, i18n — **erledigt** | 1–2 Wochen | 0 |
| 2 | Engine-Events, Live-Debug, Node-Status | 1 Woche | 0 |
| 3 | Schema v2, Edit-Tray v2, Widgets, dynamische Ports | 2 Wochen | 1, 2 |
| 4 | Visuelles Redesign, Tokens v2, Icons, Dark Mode | 2 Wochen | 1, 3 |
| 5 | Editor-Ergonomie, Node-RED-Import | 1–2 Wochen | 1, 4 |
| 6 | Backend-Reife | 2 Wochen | 0 (parallel) |
| 7 | Auslieferung, Doku | 1 Woche | alle |

Gesamt: ca. 11–13 Personenwochen bis zu einem Stand, den man öffentlich zeigen kann. Phasen 0+1+2
(3–4 Wochen) liefern bereits ein funktionierendes, ehrliches Produkt; Phase 4 macht es
ansehnlich.

---

## 5. Definition of Done (Abnahmekriterien)

- **Funktional**: Flow anlegen → Inject + Function + Debug platzieren → verbinden → Deploy →
  Nachricht erscheint innerhalb von 2 s im Debug-Tab, ohne Klick auf "Refresh". Node
  verschieben → Reload → Position bleibt. Redeploy ändert Verhalten sofort.
- **Technisch**: genau eine WebSocket-Verbindung pro Tab; Browser-Console in Produktion leer;
  `gofmt -l .` leer; `golangci-lint` grün; `go test -race ./...` grün; `npm run lint && npm test
  && npm run build` grün; Playwright-Smoke in CI grün; kein Binary im Repo; Docker-Image baut.
- **Visuell**: alle Text/Hintergrund-Kombinationen ≥ 4,5 : 1 (Kontrast-Check als Test über die
  Token-Datei); 10 Kategorien auf den ersten Blick unterscheidbar; ein Icon-Set; Light und Dark
  Mode vollständig; Lighthouse Accessibility ≥ 90.
- **Editor**: Undo/Redo, Copy/Paste, Multi-Select, Kontextmenü, Shortcuts, Node-Suche vorhanden
  und getestet; Switch mit 5 Regeln zeigt 5 Ports.
- **Doku**: `PROTOCOL.md`, `ARCHITECTURE.md`, README mit Screenshots aktuell;
  `AGENTS.md`-Dateien beschreiben nur, was existiert.

---

## 6. Entscheidungen, die der Maintainer treffen sollte

1. **Bruch mit "nur drei Go-Farben" für Kategorien?** Empfehlung: **ja**. Go-Farben bleiben
   Marke (Akzent, Wortmarke, Danger), Kategorien bekommen eigene Hues. Ohne diese Entscheidung
   ist Punkt 1.3.3 nicht lösbar.
2. **Ein Schreibpfad (Deploy = Speichern + Aktivieren wie Node-RED) statt Auto-Sync per
   WebSocket?** Empfehlung: **ja**; das macht Undo, Dirty-State und Konfliktfreiheit trivial.
3. **`@xyflow/react` 12 jetzt migrieren?** Empfehlung: **ja, in Phase 1**, bevor Editor-Features
   darauf aufbauen; die Migration ist mechanisch (Imports, `nodeOrigin`, Handle-Typen).
4. **Icon-Namen statt SVG-Markup über die API?** Empfehlung: **ja** (Lucide); bricht die
   `icon`-Semantik der Node-Metadaten, ist aber ein Einzeiler pro Node.
5. **Default-Sprache Englisch, Deutsch als Übersetzung?** Empfehlung: **ja**; Codebasis,
   Node-Namen und Zielgruppe (Node-RED-Nutzer) sind englisch.
6. **Node-RED-Import als Ziel?** Empfehlung: **ja, in Phase 5**; es ist der stärkste Grund, das
   Projekt auszuprobieren.
7. **Repo-Historie von den 20-MB-Binaries befreien (`git filter-repo`)?** Kosmetisch, aber
   dauerhaft 10 MB weniger pro Clone; erfordert Force-Push und Koordination.

---

## Anhang A — Backend-Audit (Concurrency, Persistenz, Sicherheit)

Ergebnis eines separaten Code-Audits des Go-Backends (alle Punkte im Code nachgelesen, die mit
"reproduziert" markierten zusätzlich ausgeführt).

### A.1 Nachrichtenpfad Inject → Debug (`internal/engine/engine.go`)

1. `InjectMessage` (743–769) legt eine `Message` mit `Path=[injectNodeID]` auf den per-Flow
   `activeFlow.msgChan`.
2. `processFlowMessages` (649–661, eine Goroutine pro Flow) ruft `processMessage` synchron.
3. `processMessage` (248–298): `AddMessageToLog` (globale Mutex, 863–874) → `log.Printf` pro
   Nachricht (252) → `findTargetNodes` (lineare Suche über `flow.Connections`, 422–436) →
   **eine Goroutine pro Ziel-Node** (287), jede mit `msg.Clone()` (Map- und Slice-Kopien,
   `message.go:449–460`), `context.WithTimeout` und einer frischen `NodeRuntime` mit zwei
   Closures (599–615).
4. Ausgaben gehen per `submitMessage` (439–452) auf den **engine-weiten** `e.msgChan`
   (non-blocking, verwirft bei vollem Puffer). Diesen Kanal lesen **101 konkurrierende Leser**:
   100 `worker()`-Goroutinen (217–227) plus `processMessages()` (230–245), und jeder davon startet
   pro Nachricht *wieder* eine Goroutine. Effektive Nebenläufigkeit: unbegrenzt.
5. Pro Hop: 2 Goroutine-Starts, ~6 Allokationen, 1 globaler Mutex-Log-Write, 1 globaler
   Mutex-Log-Append.

Namens-Falle: `engine.Message.Payload` ist das ganze Node-RED-`msg`-Objekt; `msg["payload"]` ist
die eigentliche Nutzlast (`message.go:374`).

### A.2 Concurrency

- Hub-Race und Panic ohne `recover()` (B14).
- `*engine.Flow` als geteilter, ungeschützter Zustand zwischen Handlern, Workern und `SaveFlow`
  (`integration.go:400, 435, 484–489`; `main.go:293`; `dto/convert.go:289–308`; `engine.go:264–276,
  401–436`; `state/manager.go:33`).
- Lifecycle: `CreateFlow` legt einen Stub ohne `ctx/cancel/executors` in `e.flows` (780–788);
  `Deploy` verweigert (475–478); `Undeploy` panict (674); REST kaschiert es (B1), WS liefert
  Fehler (`integration.go:332`). Es gibt keinen Redeploy-Pfad: `flow:update` mutiert die Config,
  Executors werden nie neu initialisiert. `Undeploy` löscht den Flow aus `e.flows` (684) →
  undeployte Flows verschwinden aus `GetAllFlows` bis zum Neustart.
- Shutdown: B17. `Undeploy` wartet auf `EmittingNode.Start`, nicht auf laufende
  `processMessage`-Goroutinen.
- `join`-Gruppen laufen nie ab (dokumentiert, `join/node.go:240–244`); `delay` blockiert eine
  Goroutine pro Nachricht für die volle Dauer (`delay/node.go:68–80`); EventBus-Handler laufen
  synchron auf der publizierenden Goroutine (`eventbus.go:48–52`).

### A.3 API / Protokoll

- Bestes Stück des Backends: `internal/dto` + `registry.NodeMetadata` + `websocket.MessageType`
  → `cmd/gentypes` → `generated.ts` mit CI-Staleness-Check. Darauf aufbauen.
- Nichts wird vom Server initiiert gepusht (B5). `dto.NodeToWire` setzt Status hart auf `idle`.
- `RequestID` existiert im Envelope (`hub.go:89`), wird in Antworten nie gesetzt → keine
  Request/Response-Korrelation.
- REST und WS unterscheiden sich semantisch (Deploy; REST versteckt Fehler, WS sendet
  `err.Error()` roh — `integration.go:226, 240, 318, 337, 404`, trotz Commit 23d258d).
- `node:config` rebroadcastet ohne zu persistieren (`integration.go:507–515`); `flow:list` wird
  nach jedem Node-Drag komplett neu gesendet; `generateID` ist tot (676).
- Zwei Status-Enums (`engine.FlowStatus` inactive/active vs. `dto.FlowStatus` draft/running);
  `ActiveFlow.Status` und `Flow.Status` driften; `deploying/undeploying` werden nie gesetzt.

### A.4 Node-Modell (`internal/registry/registry.go:110–148`)

- TypedInputs (`msg/flow/global/str/num/json/env`) sind `Type:"object"` mit Prosa-Beschreibung
  (`switchnode/node.go:555–559`, `httprequest/node.go:365`).
- Config-Node-Referenzen (`tls`, `proxy`, `broker`) sind `Type:"string"` + Prosa; "config"-Nodes
  sind nicht als "nicht auf dem Canvas" markiert.
- Keine Item-Schemas für `rules`-Arrays; keine Output-Anzahl, keine dynamischen Outputs (B10).
- Properties sind eine Go-Map → JSON-Reihenfolge zufällig; keine Labels, Gruppen, Widget-Hinweise.
- `SetConfig` ignoriert falsch typisierte Werte stumm; unbekannte Node-Typen werden akzeptiert
  und scheitern erst beim Deploy.

### A.5 Persistenz (`internal/state/manager.go`)

Siehe B19. Zusätzlich: Speichern bei jeder WS-Mutation (inkl. Node-Drag) ohne Debounce
(`integration.go:409, 444, 495, 549, 583`); Flow-/Global-Kontext nur im Speicher.

### A.6 Sicherheit

- Keine Authentifizierung; `CheckOrigin` liefert `true` (`hub.go:327–331`) → Cross-Site-
  WebSocket-Hijacking der lokalen Instanz durch jede Webseite.
- Path-Traversal über Flow-IDs (B18).
- Kein `http.MaxBytesReader` auf REST-Bodies (`main.go:234, 268, 450`); WS hat 512 KB Limit.
- Positiv: `exec`-Node ohne Shell, mit argv, Timeout, Output-Cap und Env-Opt-in
  (`execnode/node.go:1–30`). `httprequest` mit Timeouts/Size/Redirect-Caps, SSRF bewusst nicht
  mitigiert (dokumentiert). File-Nodes ohne Root-Jail (wie Node-RED, aber ohne Auth davor).

### A.7 Hygiene

- gofmt: 130 von 135 Go-Dateien unformatiert; `dto/` und `gentypes` nutzen Tabs → Baum in sich
  inkonsistent.
- Dockerfile: Go 1.21 vs. 1.25, kopiert das gitignorierte `web/dist`, `ENV PORT/DATA_DIR` ungenutzt.
- 96 `log.Printf` mit Ad-hoc-Präfixen, doppelte Zeilen (`engine.go:497–498, 500–501, 513–514`).
- `integration.go` mit anonymen Struct-Parametern; CI ohne `-race` (Commit 910c062).

### A.8 Tests

`go test ./...`: 55 Pakete ok, 3 ohne Tests (`gentypes`, **`nodes/function`**, `flatted`),
~8 s. 450 Testfunktionen, 0 Benchmarks, 1 `t.Skip`, 17 Testdateien mit `time.Sleep`. Stärkste
Tests: `engine/phase*_test.go` (echtes Deploy → Inject → Assert). Ungetestet: Create→Deploy,
Undeploy eines angelegten Flows, gleichzeitige Edits, Shutdown, korrupte/atomare Persistenz,
Path-Traversal, WS-Fehlerpfade, `HandleMessage` insgesamt.

### A.9 Priorisierte Backend-Maßnahmen (fließen in Phase 0, 2, 6 ein)

| # | Maßnahme | Aufwand |
|---|---|---|
| 1 | Flow-Lifecycle trennen: bekannt vs. deployed; Create→Deploy→Edit→Redeploy→Undeploy ohne Panic, ohne Verschwinden | L |
| 2 | Unveränderliche Deploy-Snapshots; Mutation nur über Engine-Methoden unter Lock | M |
| 3 | Dispatch neu: ein Leser, begrenzte Nebenläufigkeit (Semaphore/Per-Node-Mailbox), keine Goroutine-Paare pro Hop, kein Log pro Nachricht, IDs fixen, `submitMessage` nach `Stop` sicher; Benchmark vor jeder Durchsatz-Aussage | M |
| 4 | Runtime-Events an die UI pushen (EventBus → Hub), `RequestID` setzen, eine Nachricht pro Frame | M |
| 5 | Hub: Client-Map nur in `Run` mutieren, `send` nie außerhalb schließen, `recover()` in Pumps | S |
| 6 | Sicherheits-Basis: ID-Muster `^[A-Za-z0-9_-]+$`, echtes `CheckOrigin`, optionaler Bearer-Token, `MaxBytesReader`, keine rohen `err.Error()` über WS | S |
| 7 | NodeMetadata v2 (siehe Phase 3) | M |
| 8 | Persistenz: temp+rename+fsync, defekte Dateien in Quarantäne + Log, Wire-Format mit `schemaVersion` + Migration, Debounce, Draft vs. Deployed persistieren | M |
| 9 | Node-SDK-Paket: Clone, TypedValue-Codec, Resolver, Struct-basierte Config-Dekodierung, `Execute(ctx context.Context, …)` | M |
| 10 | Hygiene/CI: gofmt (Tabs) + blockierend, `-race`, Binaries raus, Dockerfile, Flags ehren oder entfernen, `slog` | S |
