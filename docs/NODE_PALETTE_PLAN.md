# Plan: Vollständige Node-Palette (Node-RED Core Parity)

## Ziel

Go-RED auf die vollständige **Core-Node-Palette** von Node-RED bringen, so wie sie in
[`packages/node_modules/@node-red/nodes/core`](https://github.com/node-red/node-red/tree/main/packages/node_modules/%40node-red/nodes/core)
des offiziellen `node-red/node-red`-Repos definiert ist. Dies ist ein **Plan-Dokument**
(analog zu `docs/FRONTEND_NODE_RED_REDESIGN.md`) — es ändert keinen Code, sondern legt
Inventar, Architektur-Lücken und Reihenfolge fest, aus denen einzelne PRs abgeleitet werden.

Referenzstand: `node-red/node-red@main`, Verzeichnis `core/` mit 6 Unterordnern und 46
registrierten Node-Typen (per `RED.nodes.registerType`, aus den `.js`-Dateien extrahiert,
Stand dieses Plans). NR-Dashboard-Nodes (`node-red-dashboard`) sind **kein** Teil von
`core` und daher nicht Gegenstand dieses Plans.

---

## Ist-Zustand in Go-RED

- Registrierte Nodes (Stand nach Phase 6): `inject`, `debug`, `function` (vor Phase 0/1),
  `junction`, `comment`, `catch`, `status`, `complete`, `link in`, `link out` (Phase 1),
  `switch`, `change`, `range`, `template`, `delay`, `trigger`, `rbe`, `exec` (Phase 2),
  `csv`, `json`, `xml`, `yaml`, `html` (Phase 3), `split`, `join`, `sort`, `batch`
  (Phase 4), `file`, `file in`, `watch` (Phase 5), `tls-config`, `http proxy`,
  `mqtt-broker`, `mqtt in`, `mqtt out`, `http in`, `http response`, `http request`,
  `websocket-listener`, `websocket-client`, `websocket in`, `websocket out`, `tcp in`,
  `tcp out`, `tcp request`, `udp in`, `udp out` (Phase 6) — **47 Node-Typen**, alle sechs
  Phasen abgeschlossen. Von den ursprünglich im Inventar erfassten Typen bleiben `link
  call` und `unknown` bewusst nicht implementiert (Begründung: Phase-1-Abschnitt unten);
  `global-config` als eigener NR-Type-ID ist nicht 1:1 nachgebaut, aber das
  Config-Node-*Konzept*, das dieser Eintrag im Inventar vertreten hat, ist mit den fünf
  Phase-6-Config-Nodes (`tls-config`/`http proxy`/`mqtt-broker`/`websocket-listener`/
  `websocket-client`) real und mehrfach genutzt umgesetzt. Rest dieses Abschnitts
  beschreibt den Stand *vor* Phase 0, s. Update-Hinweis unten.
- `registry.NodeExecutor` kennt nur `Execute/Validate/GetConfig/SetConfig`. Kein
  Lifecycle-Hook zum Aufräumen (`Close`), kein Konzept für Nodes, die selbst Events
  erzeugen statt nur auf eingehende Messages zu reagieren.
- `engine.NodeConnection` hat zwar `SourcePort`/`TargetPort` (`internal/engine/flow.go:56-63`),
  aber `findConnectedNodes` in `internal/engine/engine.go:305-317` ignoriert das bewusst:
  *"For now, we ignore ports and connect all outputs"*. Jeder Node hat de facto genau
  einen logischen Ausgang.
- `NodeExecutor.Execute` liefert eine einzelne `map[string]interface{}` zurück — kein
  Weg, um "kein Output" (RBE/Filter), "mehrere Outputs mit unterschiedlichem Payload"
  (Switch, Split) oder "Output an Port 2 statt Port 1" (Catch/Complete) abzubilden.
- Kein Flow-/Global-Context-Store (`flow.get`/`global.get`/`.set`), den Change/Switch/
  Template/Function laut NR-Semantik brauchen.
- Kein Event-Bus für Catch/Status/Complete — in NR hören diese Nodes nicht auf normale
  Wire-Verbindungen, sondern flow-weit auf Fehler/Status-Events anderer Nodes.
- Kein "Config-Node"-Konzept: In NR sind `mqtt-broker`, `tls-config`, `http proxy`,
  `websocket-listener` eigenständige, wiederverwendbare Konfigurationsobjekte, die von
  mehreren regulären Nodes per ID referenziert werden (Dropdown "Add new..." im Editor).
  `registry.NodeMetadata` kennt nur normale, in den Flow verdrahtete Nodes.
- Kategorien aktuell in `web/src/utils/nodeCategories.ts`: `input, output, function,
  social, storage, network, protocol, parser, dashboard, custom`. Passt schon gut zu den
  NR-Kernkategorien, `AGENTS.md` nennt zusätzlich `logic`/`sensor` (bisher ungenutzt).

**Konsequenz:** Ein großer Teil der 46 Nodes lässt sich nicht sauber implementieren, ohne
vorher die Engine zu erweitern. Nodes 1:1 aus NR zu portieren, ohne diese Lücken zu
schließen, würde nur oberflächlich funktionierende Nodes erzeugen (z.B. ein `switch`, der
immer an alle Outputs sendet). Der Plan sieht daher **Phase 0 (Engine-Grundlagen)** vor
allen Node-Implementierungen vor.

> **Update:** Diese Analyse ist der Stand *vor* Phase 0. Alle hier beschriebenen Lücken
> außer dem Config-Node-Konzept sind inzwischen geschlossen — siehe Status in
> "Architektur-Voraussetzungen (Phase 0, blockierend)" unten.

---

## Vollständiges Node-Inventar (46 Typen, 6 NR-Kategorien)

Spalte "Go-RED-Kategorie" = Zielwert für `NodeMetadata.Category`, abgeglichen mit
`CATEGORY_ORDER` in `web/src/utils/nodeCategories.ts` (neue Kategorie `flow-control` muss
dort ergänzt werden, siehe Phase 1).

### `common/` → primär `flow-control`, `input`, `output`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Voraussetzung |
|---|---|---|---|---|
| `inject` | `20-inject.js` | ✅ bereits vorhanden | input | — |
| `debug` | `21-debug.js` | ✅ bereits vorhanden | output | — |
| `junction` | `05-junction.js` | ✅ Wire-Kreuzung ohne Funktion (nur Editor-Routing) | flow-control | — |
| `complete` | `24-complete.js` | ✅ Feuert, wenn ein anderer Node fertig ist | flow-control | Event-Bus (Phase 0) |
| `catch` | `25-catch.js` | ✅ Fängt Fehler anderer Nodes ab | flow-control | Event-Bus (Phase 0) |
| `status` | `25-status.js` | ✅ Reagiert auf Statusänderungen anderer Nodes | flow-control | Event-Bus (Phase 0) |
| `link in` | `60-link.js` | ✅ Virtueller Einstiegspunkt für `link out` (selber Flow) | flow-control | `NodeRuntime.SubmitToNode` |
| `link out` | `60-link.js` | ✅ Liefert direkt an `link in`-Node(s) im selben Flow; **kein Cross-Flow** | flow-control | `NodeRuntime.SubmitToNode` |
| `link call` | `60-link.js` | ⏳ nicht implementiert — braucht Request/Response-Korrelation+Timeout | flow-control | eigene Infrastruktur |
| `comment` | `90-comment.js` | ✅ Reiner Editor-Kommentar, keine Laufzeitfunktion | flow-control (no-op) | — |
| `global-config` | `91-global-config.js` | ⏳ verschoben auf Phase 6 | — (config-node) | Config-Node-Konzept |
| `unknown` | `98-unknown.js` | ⏳ nicht geplant (Editor-Platzhalter, geringer Wert ohne Type-Migration-UX) | — (internal) | — |

### `function/` → `function`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Voraussetzung |
|---|---|---|---|---|
| `function` | `10-function.js` | ✅ bereits vorhanden (Goja statt Duktape) | function | — |
| `switch` | `10-switch.js` | ✅ Routing nach Bedingungen auf mehrere Outputs (Paket `switchnode`) | function | Multi-Output-Routing |
| `change` | `15-change.js` | ✅ Set/Change/Delete/Move auf msg/flow/global | function | Context-Store, Typed Properties |
| `range` | `16-range.js` | ✅ Numerischen Wertebereich skalieren (Paket `rangenode`) | function | — |
| `template` | `80-template.js` | ✅ Variablen-Substitution `{{msg.pfad}}` (kein Mustache-Sections/Partials, s.u.) | function | eigener Mini-Renderer (kein neues Modul) |
| `delay` | `89-delay.js` | ✅ Fixe Verzögerung + Rate-Limit (Queue/Drop) | function | `Closeable`/`EmittingNode` (Phase 0) |
| `trigger` | `89-trigger.js` | ✅ Sendet sofort, optional zweite Message nach Delay (kein Reset, s.u.) | function | `Closeable`, `NodeRuntime.SubmitToNode` |
| `exec` | `90-exec.js` | ✅ Nur argv (nie Shell), Opt-in per Env-Var, Timeout+Output-Cap (Paket `execnode`) | function | Security-Review — s. eigener Abschnitt unten |
| `rbe` (`filter`) | `rbe.js` | ✅ `rbe` (exakte Änderung) + `deadband` (numerischer Gap); kein `narrowband` | function | `MultiOutputExecutor` (Phase 0) |

### `parsers/` → `parser`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Go-Bibliothek |
|---|---|---|---|---|
| `csv` | `70-CSV.js` | ✅ CSV ↔ Array (Objekte oder Arrays je nach Header-Konfiguration) | parser | stdlib `encoding/csv` |
| `html` | `70-HTML.js` | ✅ CSS-Selector-Subset (Tag/`.class`/`#id`/Nachfahren) statt vollem cheerio | parser | `golang.org/x/net/html` (jetzt direkt) |
| `json` | `70-JSON.js` | ✅ JSON ↔ Objekt, Richtung anhand `payload`-Typ | parser | stdlib `encoding/json` |
| `xml` | `70-XML.js` | ✅ XML ↔ Objekt, eigenes Schema (`@attr`/`#text`/Arrays), nicht xml2js-kompatibel | parser | stdlib `encoding/xml` |
| `yaml` | `70-YAML.js` | ✅ YAML ↔ Objekt | parser | `gopkg.in/yaml.v3` (jetzt direkt) |

### `sequence/` → `flow-control`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Voraussetzung |
|---|---|---|---|---|
| `split` | `17-split.js` | ✅ Array/Objekt/String in Teil-Messages zerlegen (kein Buffer-Split) | flow-control | `msg.parts` als normaler Payload-Key |
| `join` | `17-split.js` | ✅ Teil-Messages wieder zusammenführen (kein Timeout-Flush) | flow-control | `msg.parts`, Stateful |
| `sort` | `18-sort.js` | ✅ Sequenz von Messages nach Property sortieren | flow-control | `msg.parts`, Stateful |
| `batch` | `19-batch.js` | ✅ Count- oder Interval-basiertes Batching (kein Overlap) | flow-control | Stateful, `EmittingNode` (Phase 0) |

### `network/` → `network` / `protocol`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Go-Bibliothek |
|---|---|---|---|---|
| `tls-config` | `05-tls.js` | TLS-Konfiguration (Cert/Key/CA) | — (config-node) | stdlib `crypto/tls` ✅ |
| `http proxy` | `06-httpproxy.js` | Proxy-Konfiguration für HTTP-Nodes | — (config-node) | ✅ |
| `mqtt-broker` | `10-mqtt.js` | MQTT-Broker-Verbindung | — (config-node) | `eclipse/paho.mqtt.golang` ✅ |
| `mqtt in`/`mqtt out` | `10-mqtt.js` | MQTT Subscribe/Publish | network | `eclipse/paho.mqtt.golang` ✅ |
| `http in`/`http response` | `21-httpin.js` | Eigenen HTTP-Endpunkt im Flow definieren | network | stdlib `net/http` ✅ |
| `http request` | `21-httprequest.js` | Ausgehende HTTP-Requests | network | stdlib `net/http` (+ SSRF-Review) ✅ |
| `websocket-listener`/`websocket-client` | `22-websocket.js` | WS-Config-Nodes | — (config-node) | `gorilla/websocket` ✅ |
| `websocket in`/`websocket out` | `22-websocket.js` | WS senden/empfangen | network | `gorilla/websocket` ✅ |
| `tcp in`/`tcp out`/`tcp request` | `31-tcpin.js` | TCP Server/Client | network | stdlib `net` ✅ |
| `udp in`/`udp out` | `32-udp.js` | UDP Server/Client | network | stdlib `net` ✅ |

### `storage/` → `storage`

| NR-Type-ID | Datei | Zweck | Go-RED-Kategorie | Go-Bibliothek |
|---|---|---|---|---|
| `file`/`file in` | `10-file.js` | Datei schreiben/lesen | storage | stdlib `os` ✅ |
| `watch` | `23-watch.js` | Dateisystem-Änderungen beobachten | storage | `fsnotify/fsnotify` ✅ |

**Zusammenfassung Aufwand:** 43 neue Node-Typen (46 minus die 3 vorhandenen), davon
6 reine Config-Nodes ohne eigenen Message-Flow, 2 interne/Editor-only (`comment`,
`unknown`) ohne echte Laufzeitlogik. Neue Go-Abhängigkeiten: `paho.mqtt.golang`,
`fsnotify`. Alles andere deckt die Stdlib oder bereits vorhandene Module ab.

---

## Architektur-Voraussetzungen (Phase 0, blockierend)

Muss vor den meisten Nodes in `function/`, `common/`, `sequence/` und `network/`
umgesetzt sein, sonst entstehen Nodes, die nur oberflächlich der NR-Semantik entsprechen.
**Status: 1–7 sind implementiert** (siehe Dateien unten); 8 ist bewusst auf Phase 6
verschoben (Begründung dort). Alles additiv über optionale Interfaces — die 3
bestehenden Nodes (`inject`/`debug`/`function`) sind unverändert, keine
`NodeMetadata`/`Port`/`Property`/`Schema`-Änderung, also kein Frontend-Regen nötig.

1. ✅ **Multi-Output-Routing nach Port** — `registry.MultiOutputExecutor` (optionale
   Erweiterung von `NodeExecutor` um `ExecuteMulti`, ein Payload pro Output-Port-ID,
   fehlender/`nil`-Port = kein Send). `findConnectedNodes` in `internal/engine/engine.go`
   filtert jetzt nach `conn.SourcePort`, aber nur wenn sowohl die Message einen
   `OutputPort` trägt als auch die Connection einen `SourcePort` gesetzt hat — leere
   Ports (jeder bisherige Flow/Test) verhalten sich unverändert wie vorher (Broadcast an
   alle Outputs). Betrifft künftig: `switch`, `catch`, `complete`, `split`, `rbe`,
   `tcp request`.
2. ✅ **Node-Lifecycle `Close()`** — `registry.Closeable`. `closeNodeExecutors` in
   `internal/engine/engine.go` ruft `Close()` für jeden Executor auf, der es implementiert,
   sowohl bei `Undeploy` als auch beim Abbruch eines fehlgeschlagenen `Deploy` (bereits
   initialisierte Nodes werden vor dem Fehler-Return aufgeräumt). Betrifft künftig:
   `mqtt in/out`, `tcp in`, `udp in`, `websocket in`, `watch`, `delay`.
3. ✅ **Event-erzeugende Nodes** — `registry.EmittingNode` (`Start(ctx, emit)`).
   `FlowEngine.startEmittingNodes` startet für jeden solchen Node eine Goroutine beim
   Deploy, getrackt über das bereits vorhandene (zuvor ungenutzte) `ActiveFlow.wg`, sodass
   `Undeploy` wartet, bis alle `Start`-Aufrufe die Context-Cancellation gesehen haben und
   zurückgekehrt sind, bevor `Close()` aufgerufen wird. Betrifft künftig: `tcp in`,
   `udp in`, `http in`, `websocket in`, `mqtt in`, `watch`.
4. ✅ **Flow-weiter Event-Bus** — `registry.EventBus` (`OnError`/`OnStatus`/`OnComplete`/
   `PublishError`/`PublishStatus`/`PublishComplete`; ursprünglich in `internal/engine`
   gebaut, in Phase 1 nach `internal/registry` verschoben — Begründung dort), ein
   Exemplar pro `ActiveFlow`, per `FlowEngine.GetFlowContext(flowID)` erreichbar. Jeder
   `Execute`/`ExecuteMulti`-Aufruf publiziert automatisch ein `NodeErrorEvent` (Fehler)
   oder `NodeCompleteEvent` (Erfolg); Nodes können zusätzlich über
   `NodeRuntime.ReportStatus`/`ReportError` selbst publizieren. Konsumiert von `catch`,
   `status`, `complete` (Phase 1).
5. ✅ **Flow-/Global-Context-Store** — `registry.ContextStore` (thread-safe Get/Set/
   Delete/Keys; ebenfalls in Phase 1 nach `internal/registry` verschoben). Jeder
   `ActiveFlow` hat einen privaten (`ContextStore`-Feld), die `FlowEngine` einen globalen
   (`GlobalContext()`). Erreichbar aus `Execute`/`ExecuteMulti`/`Start` über
   `registry.RuntimeFromContext(ctx)` → `NodeRuntime.FlowContext`/`.GlobalContext`.
6. ✅ **Typed Properties** — neues, eigenständiges Package `internal/typedvalue`
   (`Value{Type, Value}.Resolve(Resolver)`), Typen `str/num/bool/json/env/msg/flow/global`
   (kein `jsonata`, s.u.). Entkoppelt von `internal/engine` über das strukturelle
   `ContextGetter`-Interface (das `*registry.ContextStore` bereits erfüllt) — kein Import-
   Zyklus. Erster Konsument in Phase 2 (`switch`/`change`), seitdem auch von
   `range`/`split`/`sortnode` genutzt.
7. ✅ **`msg.parts`-Metadaten — Design-Korrektur gegenüber der ursprünglichen Planung.**
   Kein eigenes `engine.Message`-Feld (das ursprüngliche `Message.Parts *MessageParts`
   aus Phase 0 hatte nie einen Producer/Consumer und wurde in Phase 4 wieder entfernt).
   Stattdessen: `msg.parts` ist ein ganz normaler Schlüssel derselben
   `map[string]interface{}`, die für `msg.payload`/`msg.topic`/etc. schon verwendet wird
   (`input["parts"] = map[string]interface{}{"id","index","count","type",...}`) — exakt
   das Muster, das `change`/`switch` bereits für `msg.topic` etc. nutzen. Kein
   Engine-Change nötig; `split`/`join`/`sortnode` lesen/schreiben das rein auf
   Node-Ebene. Siehe `internal/nodes/split`'s Package-Doc für das genaue Schema.
8. ⏳ **Config-Node-Konzept — auf Phase 6 verschoben.** Ohne einen echten Config-Node
   (z.B. `mqtt-broker`) als Referenzimplementierung lässt sich das Wire-Format
   (`internal/dto`/`generated.ts`: wie referenziert ein Node einen Config-Node, wie
   unterscheidet die UI Config- von Flow-Nodes in der Palette) nicht sinnvoll festlegen,
   ohne zu raten. Wird zusammen mit `tls-config`/`mqtt-broker` zu Beginn von Phase 6
   entworfen und umgesetzt, nicht vorab spekulativ.

Dateien: `internal/registry/registry.go` (neue optionale Interfaces),
`internal/engine/{engine,message,context_store,eventbus,runtime}.go`,
`internal/typedvalue/typedvalue.go`. Tests: `internal/registry/registry_test.go`
(`TestOptionalNodeInterfaces`), `internal/engine/{phase0,context_store,eventbus,
runtime}_test.go`, `internal/typedvalue/typedvalue_test.go`.

Da keine der oben genannten Änderungen `NodeMetadata`/`Port`/`Property`/`Schema` in
`internal/registry/registry.go` oder irgendetwas unter `internal/dto/**` betrifft, greift
die **Interface-Verifikation** aus `internal/nodes/AGENTS.md` hier nicht (kein
`go generate`/`tsc`/`npm test` nötig) — geprüft wurde trotzdem mit `go build ./...`,
`go vet ./...` und `go test ./... -race` für `internal/engine`, `internal/registry`,
`internal/typedvalue`. Sobald Config-Nodes (Punkt 8) oder eine erste Node mit
`MultiOutputExecutor`/Typed-Properties-Konfiguration (Phase 1/2) tatsächlich neue
`NodeMetadata`-Felder brauchen, greift die Interface-Verifikation wieder und muss
durchlaufen werden.

---

## Phasenplan

Reihenfolge nach Abhängigkeit, nicht nach NR-Ordnernamen. Jede Phase ist ein eigener PR
(oder mehrere kleine PRs); "Milestone" beschreibt das Abnahmekriterium.

### Phase 0 — Engine-Grundlagen (blockierend, s.o.) — ✅ abgeschlossen (Punkte 1–7)
Multi-Output-Routing, `Close()`-Lifecycle, Event-erzeugende Nodes, Event-Bus,
Context-Store, Typed Properties, `msg.parts` sind implementiert und getestet;
Config-Node-Konzept (Punkt 8) verschoben auf Phase 6 (Begründung oben).
**Milestone:** bestehende 3 Nodes laufen unverändert weiter (Regressionstests grün),
neue Engine-APIs haben Unit-Tests, aber noch keine neuen Nodes. — **Erreicht.**

### Phase 1 — `common/` Flow-Control — ✅ abgeschlossen (außer `link call`, `global-config`, `unknown`)
Implementiert: `junction`, `comment` (No-op), `complete`, `catch`, `status`, `link in`,
`link out` (`internal/nodes/{junction,comment,complete,catch,status,linkin,linkout}`).
Kategorie `flow-control` in `web/src/utils/nodeCategories.ts` ergänzt (Farbe
`gr-fuchsia-600`, eingeordnet nach `function`).

`catch`/`status`/`complete` sind als `registry.EmittingNode` implementiert: sie haben
kein Input-Wire (wie im echten NR-Editor), sondern abonnieren in `Start` über die neuen
`NodeRuntime.OnError`/`OnStatus`/`OnComplete`-Methoden das `EventBus` ihres Flows,
gefiltert nach einem `scope []string` (leer = alle Nodes im Flow). Dafür wurde der
Phase-0-EventBus um `NodeCompleteEvent`/`OnComplete`/`PublishComplete` erweitert; die
Engine publiziert nach jedem erfolgreichen `Execute`/`ExecuteMulti` automatisch ein
Complete-Event (`engine.go: handleNodeComplete`).

`link out` nutzt eine neue `NodeRuntime.SubmitToNode(nodeID, payload)`-Fähigkeit, die
eine Message direkt an einen Ziel-Node im selben Flow ausliefert, so als hätte dieser
sie gerade selbst erzeugt (exakt wie `link in`, das in NR ebenfalls keinen echten
Input-Port hat). **Cross-Flow-Linking (NRs Haupt-Use-Case: zwischen Tabs) wird noch
nicht unterstützt** — Ziel muss eine `link in`-Node-ID im selben Flow sein; siehe
"Offene Fragen" unten.

**`link call` bewusst nicht implementiert:** NR erwartet dafür Request/Response-
Korrelation (Call wartet mit Timeout auf eine passende `link out`-im-"return"-Modus aus
dem aufgerufenen Zweig) — eine eigene Infrastruktur, die noch nicht existiert und die
zusammen mit einem konkreten Anwendungsfall entworfen werden sollte statt spekulativ
jetzt. `global-config` bleibt Teil des in Phase 6 verschobenen Config-Node-Konzepts
(Phase 0, Punkt 8). `unknown` (NRs Editor-Platzhalter für nicht installierte Typen) ist
ohne Undeploy/Redeploy-Type-Migration-UX wenig wertvoll und bleibt aus.

**Wichtige Falle beim Verwenden von `complete` mit leerem Scope:** anders als
`catch`/`status` reagiert `complete` auf genau das Event, das *jeder* erfolgreiche Node
erzeugt — auch der eigene nachgeschaltete Sink-Node. Ein leerer Scope
("beobachte alle Nodes") plus eine Wire-Verbindung von `complete` zurück in einen
beobachteten Node erzeugt eine Endlosschleife (beim Schreiben des End-to-End-Tests
tatsächlich reproduziert, siehe `internal/nodes/complete/node.go`-Doc-Kommentar und
`internal/engine/phase1_flow_control_test.go`). Immer gezielt scopen.

**Milestone:** Fehlerbehandlung (`catch`) und Status-Propagation end-to-end in einem
Test-Flow nachweisbar — **erreicht**, siehe
`internal/engine/phase1_flow_control_test.go` (`TestPhase1_FlowControlNodesEndToEnd`),
das alle sieben Nodes über eine echte deployte Flow gegen einen echten `FlowEngine`
verdrahtet (isolierte Registry, keine Mocks der Nodes selbst).

### Phase 2 — `function/` Transform & Control — ✅ abgeschlossen
`switch`, `change`, `range`, `template`, `delay`, `trigger`, `rbe`, `exec` sind
implementiert und getestet (`internal/nodes/{switchnode,change,rangenode,template,
delay,trigger,rbe,execnode}`). `internal/typedvalue` um `PropertyRef` erweitert
(Get/Set/Delete auf msg-Pfad oder Flow-/Global-Context-Key) — erster echter Konsument
der in Phase 0 vorbereiteten Typed-Properties-Infrastruktur, genutzt von `switch`
(Property-Test) und `change` (alle vier Actions).

**Scope-Cuts (dokumentiert, nicht vergessen):**
- `switch`: `hask`/`istype`-Operatoren fehlen (einfach nachrüstbar, niedriger Wert für
  Phase 2). Kein `jsonata`.
- `template`: nur `{{msg.pfad}}`-Variablensubstitution, keine Mustache-Sections
  (`{{#each}}`), Partials, oder Flow-/Global-/Env-Lookups innerhalb des Templates —
  eigener Mini-Renderer statt einer neuen Abhängigkeit.
- `delay`: nur feste Verzögerung + Rate-Limit (Queue oder "nur letzte Message"); NRs
  "random delay" und "delay each message additionally" fehlen.
- `trigger`: **kein Reset** — jede auslösende Message bekommt ihren eigenen,
  unabhängigen Timer für die zweite Message; eine neue Message kann den Timer einer
  früheren nicht abbrechen. Multi-Topic-Tracking wie bei NR fehlt ebenfalls.
- `rbe`: nur `rbe` (exakte Änderung, deep-equal) und `deadband` (absoluter numerischer
  Gap) implementiert; NRs `narrowband`-Modi und prozentualer Gap fehlen.

**`exec`-Security-Review (Punkt aus dem ursprünglichen Plan):**
- **Nie eine Shell.** `exec.CommandContext(ctx, Command, args...)` mit Args als
  Go-Slice, niemals `sh -c "..."` — anders als NRs Default (`useSpawn:false`, das über
  eine Shell läuft und daher Shell-Metazeichen interpretiert). Damit ist Shell-Injection
  über `msg.payload` strukturell ausgeschlossen, nicht nur durch Escaping-Disziplin
  (durch einen dedizierten Test mit `; rm -rf / #\`id\`$(whoami)` als Payload verifiziert
  — landet als einzelnes, wörtliches Argv-Element).
- `Command` kommt ausschließlich aus der Node-Konfiguration (Deploy-Zeit), nie aus der
  Message — ein kompromittierter/böswilliger Upstream-Node kann also nicht die
  auszuführende Datei ändern, nur (falls `appendPayload` aktiv ist) zusätzliche
  Argumente beisteuern.
- Opt-in per Umgebungsvariable `GORED_ENABLE_EXEC` auf dem Go-RED-Prozess — ohne diese
  verweigert der Node jede Ausführung. Ein frisches Deployment kann also nicht "aus
  Versehen" Kommandos ausführen.
- `TimeoutMs` (Default 30s) und ein 1-MiB-Cap auf stdout/stderr pro Lauf, gegen
  hängende/durchgehende Prozesse bzw. Speicherverbrauch.
- **Nicht gelöst und nicht lösbar durch Implementierung allein:** Wer Flows deployen
  kann (Editor/API-Zugriff), kann bei aktiviertem `exec`-Node Code mit den Rechten des
  Go-RED-Prozesses ausführen — das ist die Kernfunktion des Nodes, exakt wie bei NRs
  eigenem `exec`-Node, und wird durch keine der obigen Maßnahmen aufgehoben. Zugriff auf
  Editor/API muss entsprechend geschützt werden (außerhalb des Scopes dieses Nodes).

**Milestone:** Switch routet korrekt auf mehrere Outputs, Change liest/schreibt
Flow-Context — **erreicht**, siehe `internal/engine/phase2_function_test.go`
(`TestPhase2_SwitchAndChangeEndToEnd`): ein `change`-Node zählt Aufrufe in echtem
Flow-Context hoch, ein nachgeschalteter `switch`-Node routet abhängig vom
zurückgelesenen Wert auf den korrekten von zwei Outputs, gegen einen echten
`FlowEngine` (isolierte Registry, keine Mocks der Nodes selbst).

### Phase 3 — `parsers/` — ✅ abgeschlossen
`csv`, `json`, `xml`, `yaml`, `html` implementiert (`internal/nodes/{csvnode,jsonnode,
xmlnode,yamlnode,htmlnode}` — alle fünf mit `<name>node`-Suffix, weil der eigentliche
Node-RED-Type-ID-Name (`json`, `xml`, `yaml`, `csv`, `html`) mit dem Namen des jeweils
importierten Standard-/Drittanbieter-Pakets kollidieren würde, z.B. `package json`, das
`encoding/json` importiert). Reine Transform-Nodes ohne Engine-Abhängigkeiten aus Phase
0 (nur Standard-`Execute`), wie geplant unabhängig von Phase 1/2 entwickelbar.

`json`/`csv`/`yaml`/`xml` erkennen die Richtung anhand des `payload`-Typs (String →
parsen, Objekt/Array → serialisieren), wie im echten NR. `gopkg.in/yaml.v3` und
`golang.org/x/net/html` waren schon indirekte Abhängigkeiten (über `go.sum`), sind nach
`go mod tidy` jetzt direkte `require`-Einträge in `go.mod` — keine neuen Module.

**Scope-Cuts:**
- `xml`: eigenes, dokumentiertes Objekt-Schema (`@attr`-Präfix für Attribute, `#text`
  für Textinhalt bei Elementen mit Attributen/Kindern, Arrays für wiederholte
  gleichnamige Kind-Elemente) statt exakter xml2js-Kompatibilität. Bekannte
  Einschränkung: Geschwister-Reihenfolge *zwischen unterschiedlichen* Tag-Namen bleibt
  beim Round-Trip nicht erhalten (Kinder sind nach Tag-Name statt einer geordneten
  Liste gespeichert — derselbe Kompromiss, den die meisten XML↔JSON-Konverter eingehen);
  Reihenfolge *innerhalb* gleichnamiger Geschwister bleibt erhalten.
- `html`: eigene Mini-Selector-Engine (kein cheerio, keine neue Abhängigkeit) —
  unterstützt nur Tag-Name, `.class`, `#id` und den Nachfahren-Kombinator
  (Leerzeichen); `>`/`+`/`~`-Kombinatoren, Attribut-Selektoren (`[href]`),
  Pseudo-Klassen (`:nth-child` etc.) und kommagetrennte Selector-Listen fehlen.

**Milestone:** Round-Trip-Tests (parse → serialize → gleiche Daten) pro Format — **erreicht**,
je ein `TestNode_Execute_RoundTrip` in `internal/nodes/{jsonnode,yamlnode,csvnode,
xmlnode}` (für `html`, das nur extrahiert statt zu serialisieren, gibt es kein
Round-Trip-Konzept — stattdessen Tests für alle unterstützten Selector-Formen).

### Phase 4 — `sequence/` — ✅ abgeschlossen
`split`, `join`, `sort` (Paket `sortnode` — `sort` ist im stdlib-Paketnamen-Sinn
kollisionsträchtig, s. `internal/nodes/AGENTS.md`), `batch` implementiert
(`internal/nodes/{split,join,sortnode,batch}`).

**Architektur-Erkenntnis, die die ursprüngliche Phase-0-Planung korrigiert:** `msg.parts`
brauchte kein Engine-Feld — siehe Punkt 7 oben ("Architektur-Voraussetzungen"). Das dort
in Phase 0 vorbereitete `engine.Message.Parts`-Feld hatte nie einen Producer/Consumer
und wurde jetzt entfernt; `split`/`join`/`sortnode` behandeln `"parts"` als normalen
Payload-Map-Key.

**Neue Engine-Erkenntnis:** Ein Input-Message, die zu N Output-Messages auf demselben
Port wird (Split, und nach Sortierung auch Sort), passt nicht in das
`MultiOutputExecutor`-Modell aus Phase 0 (das genau *eine* Message pro Port erlaubt).
Statt das Interface zu erweitern (hätte alle 4 bisherigen `MultiOutputExecutor`-Nodes —
`switchnode`, `rbe`, `delay`, `linkout` — angefasst), liefern `split`/`sortnode` nur die
erste Message über den normalen `ExecuteMulti`-Rückgabewert zurück und verteilen den
Rest über `NodeRuntime.SubmitToNode(rt.NodeID, ...)` — dieselbe Technik, mit der `link
out` bereits eine andere Node erreicht, hier auf die eigene Node-ID angewendet. Kein
Engine-Change nötig.

**Scope-Cuts:**
- `split`: nur `array`/`string`/`object`-Modi; NRs Buffer-Split (feste Byte-Chunks) und
  verschachteltes Array-Splitting (`arraySplt`) fehlen.
- `join`/`sort`: **kein Timeout-Flush** — eine Gruppe, deren Teile nie vollständig
  ankommen, bleibt für die Lebensdauer des Flows im Speicher (dokumentiertes Risiko,
  kein automatisches Aufräumen).
- `batch`: kein "Overlap" (NR kann N Messages zwischen aufeinanderfolgenden Batches
  wiederholen); nur `count`- und `interval`-Modus.

**Milestone:** Array → Split → Join ergibt wieder das Original-Array in einem
Integrationstest — **erreicht**, siehe `internal/engine/phase4_sequence_test.go`
(`TestPhase4_SplitJoinRoundTrip`): ein echter `split`- und `join`-Node gegen einen
echten `FlowEngine` deployt, `["a","b","c","d"]` → vier Einzelnachrichten → wieder
`["a","b","c","d"]`, plus die Prüfung, dass `join` exakt einmal (nicht pro Teil) feuert.

### Phase 5 — `storage/` ✅ abgeschlossen
`file`, `file in`, `watch`. `watch` braucht neue Dependency `fsnotify`.
**Milestone:** Datei schreiben/lesen und Änderungen an einer beobachteten Datei lösen
einen Flow aus.

**Umsetzung:**
- `file` (`internal/nodes/file`) und `file in` (`internal/nodes/filein`) lösen ihren
  Pfad aus einem `typedvalue.Value` auf (meist ein fester String, wahlweise
  `msg`/`env`/flow/global) statt ihn direkt aus der Message zu lesen — dasselbe
  Typed-Input-Widget-Konzept, das Node-RED für dieses Feld nutzt, statt eines impliziten
  Vertrauens in Nachrichteninhalte.
- `file` unterstützt `append`/`overwrite`/`delete`, optional `createDir` und
  `appendNewline`; jeder Schreibvorgang öffnet die Datei neu (statt wie NR einen
  offenen `fs.WriteStream` über mehrere Nachrichten hinweg wiederzuverwenden) und ist
  pro Node-Instanz mit einem Mutex serialisiert, da der Engine jede Message in einer
  eigenen Goroutine ausführt (`internal/engine/engine.go`s `executeNode`).
- `file in` unterstützt `""` (rohe Bytes), `utf8` (String) und `lines` (ein Message pro
  Zeile mit `msg.parts`-Metadaten). Der `lines`-Modus nutzt denselben
  Same-Port-Fan-out über `NodeRuntime.SubmitToNode(rt.NodeID, ...)` wie `split`
  (Phase 4) — ein Input erzeugt N Outputs auf demselben Port.
- `watch` ist der erste `registry.EmittingNode` **ohne** Input-Port — `Execute`
  existiert nur, um `NodeExecutor` zu erfüllen, und liefert immer einen Fehler, da die
  Engine ihn für einen Node ohne verdrahteten Input nie aufruft. `Start` läuft mit
  `github.com/fsnotify/fsnotify`, bis der Flow-Context abgebrochen wird, und meldet
  transiente Watcher-Fehler über `NodeRuntime.ReportError` (für einen nachgeschalteten
  Catch-Node), statt durch einen Rückgabewert aus `Start` das Beenden der Emission für
  den Rest der Flow-Laufzeit zu erzwingen.
- **Dokumentierte Scope-Cuts:** `file`/`file in` unterstützen nur `utf8`/`base64` als
  Encoding (kein `iconv-lite`-Zeichensatzkatalog wie in NR); `file in`s `"stream"`-Format
  (Chunked-Reads mit High-Water-Mark) ist nicht implementiert — die gesamte Datei wird in
  den Speicher gelesen; `watch` erkennt nur `file`/`directory`/`n/a` (keine
  Block-/Character-Device-/Socket-/FIFO-Unterscheidung, da `os.FileInfo` das nicht
  portabel hergibt) und rekursives Watching ist ein einmaliger Verzeichnis-Walk beim
  Start — neu angelegte Unterverzeichnisse werden danach nicht automatisch mitverfolgt.
- Engine-Milestone-Test `internal/engine/phase5_storage_test.go`
  (`TestPhase5_WriteWatchReadRoundTrip`) deployt echte `file`/`watch`/`file in`-Nodes
  gegen eine echte `FlowEngine` in einem temporären Verzeichnis und bestätigt: ein
  `file`-Write löst den `watch`-Node aus, und ein nachfolgender `file in`-Read liefert
  exakt den geschriebenen Payload zurück.

### Phase 6 — `network/` (größter Umfang, meiste externen Abhängigkeiten) ✅ abgeschlossen
Config-Nodes zuerst: `tls-config`, `http proxy`, `mqtt-broker`,
`websocket-listener`/`-client`. Danach: `http in/response`, `http request`,
`websocket in/out`, `mqtt in/out`, `tcp in/out/request`, `udp in/out`.
`http request`/`http in` erhalten Security-Review (SSRF, Header-Injection,
Response-Size-Limits) vor Merge. Neue Dependency `paho.mqtt.golang`.
**Milestone:** MQTT- und HTTP-Roundtrip-Flow gegen einen lokalen Test-Broker/-Server.

**Umsetzung:**
- **Config-Node-Konzept (Punkt 8 aus Phase 0, hier nachgeholt):** ein Config-Node ist ein
  ganz normaler registrierter Node-Typ mit `Category: "config"` und leeren
  `Inputs`/`Outputs` — keine neue `NodeMetadata`/`Property`-Struktur nötig. Ein
  konsumierender Node referenziert ihn per ID über eine ganz normale
  `Property{Type: "string"}`-Konfigurationseigenschaft (z.B. `mqttin.Node.Broker`).
  Aufgelöst wird das zur Laufzeit über die neue Methode
  `registry.NodeRuntime.GetNode(nodeID) (NodeExecutor, bool)` (`internal/registry/
  runtime.go`, verdrahtet in `internal/engine/engine.go`s `newNodeRuntime` über
  `activeFlow.nodeExecutors`) — bewusst *lazy*, erst beim `Execute`/`ExecuteMulti`/
  `Start`-Aufruf, nicht beim `SetConfig`, weil Deploys Node-Initialisierungsschleife eine
  Go-Map durchläuft (unspezifizierte Reihenfolge) und ein Config-Node so nicht
  garantiert vor seinem Konsumenten existiert — beim ersten `Start`/`Execute`-Aufruf
  dagegen schon, da der komplette Init-Loop davor immer vollständig durchgelaufen ist.
  Der konsumierende Node castet das zurückgegebene `NodeExecutor` per Type-Assertion auf
  ein kleines, paketeigenes Interface (z.B. `mqttbroker.Broker`, `tlsconfig.Provider`) —
  ein direkter Go-Import zwischen den beiden Node-Paketen, keine Registry-vermittelte
  Abstraktion, da nur wenige Node-Typen das jeweils brauchen.
- **Ordering-Falle bei `EmittingNode`-Config-Nodes:** `engine.go`s `startEmittingNodes`
  startet für jeden `registry.EmittingNode` eine eigene Goroutine *ohne* Reihenfolge-
  Garantie zwischen ihnen. Ein Config-Node, der erst in seinem eigenen `Start` "bereit"
  würde, könnte daher von einem anderen `EmittingNode`, der ihn in dessen eigenem
  `Start` referenziert, in einer echten Race gelesen werden — anders als die reine
  Map-Iterations-Unordnung, die jeder Node ohnehin durch lazy Auflösung toleriert. Lösung:
  `mqtt-broker`/`websocket-listener`/`websocket-client` bauen ihre Verbindung
  (Client-Konstruktion, Dial, HTTP-Handler-Mount) direkt in `SetConfig` auf — läuft
  synchron in Deploys Init-Loop, immer abgeschlossen bevor irgendeine `Start`-Goroutine
  existiert — und implementieren nur `registry.Closeable`, nie selbst `EmittingNode`.
- **`mqtt-broker`/`mqtt in`/`mqtt out`** (`internal/nodes/mqttbroker`,`mqttin`,`mqttout`):
  ein geteilter `paho.mqtt.golang`-Client mit `SetAutoReconnect`/`SetConnectRetry`.
  Konsumenten registrieren sich nicht direkt per `client.Subscribe`, sondern über
  `Broker.OnConnect(handler)` — feuert sofort, falls schon verbunden, sonst bei jedem
  (Re-)Connect erneut (paho hat nur einen einzigen, Broker-weiten `OnConnectHandler`;
  dieser Node fächert ihn an alle registrierten Konsumenten auf).
- **`http in`/`http response`/`http request`** (`internal/nodes/httpin`,`httpresponse`,
  `httprequest`): alle `http in`-Nodes über alle Flows teilen sich einen einzigen
  `*http.Server` (`GORED_HTTP_NODE_PORT`, Default 1880 — bewusst getrennt vom
  `-port`-Editor/API-Server aus `cmd/go-red/main.go`, da Node-Pakete sich per `init()`
  selbst registrieren und keinen Zugriff auf dessen `mux` haben). Ein
  `*httpin.ResponseHandle` wird unter dem Message-Key `httpin.KeyResponseHandle`
  transportiert (ein ganz normaler `interface{}`-Wert, überlebt jedes Nodes flaches
  `cloneMap` unverändert) und von `http response` benutzt, um die offene HTTP-Antwort
  zu vervollständigen — dasselbe "leg einen echten Go-Wert auf die Message"-Muster wie
  `watch`. `http in`s Routing unterstützt literale Pfadsegmente und `:param` (kein volles
  Express-Routing). **Security-Review (siehe Package-Docs für Details):** `http request`
  blockiert SSRF *nicht* (Parität mit NR — Deployments mit nicht vertrauenswürdigen
  Flow-Autoren sollten Egress auf Netzwerkebene einschränken), aber Timeout,
  Response-Size-Cap (10 MiB) und Redirect-Cap (Default 5) sind aktiv; `http in`s
  geteilter `*http.Server` setzt `ReadHeaderTimeout`/`IdleTimeout` gegen Slowloris-Style
  hängende Verbindungen (net/http hat hier standardmäßig kein Timeout).
- **`websocket-listener`/`websocket-client`/`websocket in`/`websocket out`**
  (`internal/nodes/websocketlistener`,`websocketclient`,`websocketin`,`websocketout`):
  `websocket-listener` hängt sich per `httpin.RegisterHandler` (neue exportierte
  Funktion) auf denselben geteilten HTTP-Listener wie `http in`; `websocket-client`
  verbindet sich mit einer eigenen Reconnect-Loop nach außen. Beide implementieren
  `Send`/`OnMessage`, sodass `websocket in`/`websocket out` austauschbar gegen beide
  Config-Node-Typen funktionieren.
- **`tcp in`/`tcp out`/`tcp request`/`udp in`/`udp out`** (jeweils eigenes Package):
  Stdlib `net` only, kein geteiltes Config-Node-Konzept (jeder Node ist eigenständig).
  **Dokumentierte Scope-Cuts:** `tcp in` unterstützt nur einen booleschen
  Zeilen-Split (`bufio.Scanner`s Standard-`\n`-Splitting), kein konfigurierbares
  Trennzeichen/Byte-Count/Zeit-Split wie NR; `tcp out` unterstützt nur Client-Modus
  (kein Server-Broadcast-Modus, der einen geteilten Verbindungs-Pool mit `tcp in`
  bräuchte); `udp in`/`udp out` unterstützen kein Multicast und `udp out` kein
  Broadcast (bräuchte `SO_BROADCAST`, von `net.DialUDP` nicht gesetzt).
- Engine-Milestone-Tests `internal/engine/phase6_network_test.go`:
  `TestPhase6_HTTPRoundtrip` deployt zwei echte Flows (einen mit `http in`+
  `http response`, einen mit `http request`) gegen eine echte `FlowEngine` und bestätigt
  den vollen Client→Server→Client-Roundtrip; `TestPhase6_TCPRoundtrip` deployt einen
  echten `tcp in`-Node und bestätigt, dass ein echtes TCP-Socket-Schreiben eine Message
  auslöst. Ein Live-MQTT-Roundtrip-Test ist **nicht** enthalten — es gibt in dieser
  Sandbox-Umgebung keinen laufenden MQTT-Broker-Prozess; stattdessen testen
  `mqttbroker`/`mqttin`/`mqttout`s eigene Unit-Tests dieselbe Logik gegen
  `paho.mqtt.golang`s echtes `mqtt.Client`-Interface (ein echtes Go-Interface, das sich
  ohne Broker fake-implementieren lässt).

---

## Offene Fragen / Nicht-Ziele

- **Dashboard-Nodes** (`node-red-dashboard`) sind kein Bestandteil von `core` in NR und
  daher nicht Teil dieses Plans — Go-RED hat die Kategorie `dashboard` bereits reserviert,
  bleibt aber vorerst leer.
- **JSONata-Unterstützung** (in NR optional als Property-Typ verfügbar) wird hier nicht
  separat geplant; falls gewünscht, eigener Folge-Plan mit Bibliotheks-Auswahl
  (kein offizieller Go-JSONata-Port mit vollem Funktionsumfang bekannt).
- **`exec`- und `http request`-Nodes** sind sicherheitskritisch (beliebige Shell-Befehle
  bzw. beliebige ausgehende Requests aus einem Flow heraus) — brauchen vor Merge ein
  explizites Security-Review, ggf. Opt-in/Allowlist-Konfiguration auf Instanz-Ebene statt
  1:1-Port der NR-Nodes.
- **Reihenfolge Phase 3 vs. 1/2**: Parser-Nodes sind unabhängig und können vorgezogen
  werden, falls schneller sichtbarer Fortschritt gewünscht ist — sie zählen nicht zu den
  Nodes, die Phase-0-Änderungen benötigen.
