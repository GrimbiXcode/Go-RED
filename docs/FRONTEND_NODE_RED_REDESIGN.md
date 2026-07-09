# Frontend-Redesign: Node-RED Look & Feel (Go-Farbpalette)

## Ziel

Das bestehende React-WebUI von Go-RED (aktuell ein generisches Tailwind/"SaaS-Blau"-Design)
strukturell und interaktionsseitig an das originale Node-RED-Editor-UI angleichen — ohne die
Backend-Schnittstellen (`internal/dto`, WebSocket-Protokoll) oder die Domain-Logik anzufassen.
Es handelt sich um ein reines Presentation-Layer-Redesign.

**Farbschema-Entscheidung:** Layout, Komponentenstruktur und Interaktionsmuster (Header,
Flow-Tabs, Palette, Canvas, Node-Form, Sidebar-Tabs, Edit-Tray) orientieren sich an Node-RED —
die **Farbpalette selbst folgt aber nicht Node-REDs Rot-Branding, sondern der offiziellen
Go-Sprachen-Farbpalette** (Gopher Blue etc.), passend zum Projektnamen "Go-RED". Node-RED dient
also als UX-/Layout-Vorbild, Go als Farb-/Marken-Vorbild.

Kein Code wird in diesem Schritt verändert — dies ist der Plan, aus dem folgende PRs abgeleitet
werden.

---

## Ist-Zustand (Analyse)

Stack: React 18, TypeScript, ReactFlow 11, Tailwind CSS 3, Vite. `zustand` ist als Dependency
gelistet, wird aber nirgends verwendet — State läuft über `FlowProvider.tsx` (React Context um
`useFlows()`).

Komponentenübersicht (`web/src/components/`):

| Komponente | Aktuelle Rolle | Node-RED-Analogon |
|---|---|---|
| `FlowEditor.tsx` | 3-Spalten-Layout: Palette \| Canvas \| Sidebar | `red/main` Gesamtlayout |
| `Toolbar.tsx` | Weiße Toolbar: Flow-Dropdown, Save/Export/Import/Log-Buttons, Deploy/Stop, Zoom-Buttons | Header-Leiste + Deploy-Button (bei NR strikt getrennt vom Flow-Tab-Bereich) |
| `NodePalette.tsx` | Linke Spalte, Suchfeld + aufklappbare Kategorien mit Pastell-Badges | `red-palette` — exakt gleiches Konzept, aber andere Optik |
| `FlowCanvas.tsx` | ReactFlow mit hellgrauem Grid-Background, Default-Controls/MiniMap | `red-canvas` (Punktraster, eigene Zoom-Controls unten) |
| `NodeComponent.tsx` | Card mit farbigem Header-Balken (icon+label) + Body + Status-Dot im Header | NR-Node: **eine** durchgehend gefärbte Box, kein separater Header-Balken; Status als Label *unterhalb* der Node |
| `Sidebar.tsx` | Rechte Spalte, zeigt Flow- oder Node-Properties als reine Info-Liste | `red-sidebar` mit Tab-Leiste (Info/Debug/...) |
| `MessageLogPanel.tsx` | Eigenes, unabhängig ein-/ausblendbares Panel (kein festes Layout-Slot) | Sollte ein Tab *innerhalb* der Sidebar sein, nicht ein Overlay |
| `NodeConfigModal.tsx`, `ExportModal.tsx`, `ImportModal.tsx` | Zentrierte Overlay-Dialoge | NR nutzt eine seitlich einschiebende **Edit-Tray** (rechts, gleiche Breite wie Sidebar) |
| `ToastNotification.tsx` | Slide-in-Toasts oben rechts | NR-Notifications sehen ähnlich aus (kleine Boxen oben rechts) — nur Feinschliff nötig |
| `WebSocketStatus.tsx` | Auffälliges Badge | NR zeigt Verbindungsstatus dezent (Icon in Statusleiste) |

Farben/Typografie aktuell: generische Tailwind-Palette (`blue-500`, `gray-50`, …), Emoji als
Icons (📥📤🔄💾…), Standard-Systemschrift. Node-RED hat ein eigenes, konsistentes Farbsystem
pro Node-Kategorie, eigene SVG/FontAwesome-Icons und ein sehr kompaktes, dichtes UI (kleine
Schriftgrößen, wenig Whitespace im Vergleich zu typischem Tailwind-Default-Spacing).

---

## Node-RED-Referenz: Was strukturell übernommen werden soll

1. **Header** — schmale Kopfleiste (Farbe: Go-Palette statt NR-Rot, siehe unten) mit
   Logo + Titel links, Hamburger-Menü (☰) rechts, daneben der **Deploy-Button**: grau/deaktiviert
   solange keine Änderungen vorliegen, wird aktiv (Go-Akzentfarbe) sobald der Flow verändert wurde
   (mit Dropdown für "Full Deploy / Modified Flows / Modified Nodes").
2. **Flow-Tab-Leiste** direkt unter dem Header — horizontale Browser-Tab-artige Reiter pro Flow,
   mit "+"-Button zum Anlegen, Rechtsklick-Kontextmenü (statt heutigem Dropdown im Toolbar).
3. **Hauptmenü (Hamburger)** — Save/Export/Import/Manage-Palette wandern aus der Toolbar in dieses
   Menü, statt als Dauerbuttons sichtbar zu sein.
4. **Node-Palette (links)** — Suchfeld oben, darunter aufklappbare Kategorien (Pfeil-Icon,
   Kategoriename, Zähler), Nodes als kompakte Zeilen mit farbigem Swatch + Icon + Name.
   Reihenfolge der Standardkategorien: input, output, function, social, storage, analysis,
   advanced — eigene Go-RED-Kategorien danach. Die Swatch-Farben kommen aus der Go-Palette
   (siehe unten), nicht aus NRs Kategoriefarben.
5. **Canvas** — sehr helles/weißes Punktraster statt Liniengitter, Zoom-Controls unten links als
   kompakte weiße Buttongruppe (nicht ReactFlow-Default), Wires als graue Bezierkurven, die bei
   Hover/Selektion in Gopher Blue hervorgehoben werden.
6. **Node-Optik** — eine durchgehend farbige, abgerundete Box (Radius ~3-4px) je Kategorie,
   Icon links, Name zentriert/linksbündig, **kein** separater Kopfbalken. Ports als kleine
   quadratische Konnektoren exakt auf der linken/rechten Kante. Status wird als kleiner
   Punkt + Text **unterhalb** der Node angezeigt, nicht als Dot im Node-Header. Selektion =
   gestrichelte Outline in Gopher Blue.
7. **Rechte Sidebar mit Tabs** — schmaler vertikaler Tab-Streifen (Icons: Info, Debug, Dashboard,
   Context …) am rechten Rand, darunter der Inhalt. `Sidebar.tsx` (Info) und
   `MessageLogPanel.tsx` (Debug) werden zu Tabs *eines* Panels statt getrennter Komponenten.
8. **Edit-Tray statt Modal** — Node-Konfiguration öffnet sich als Panel, das von rechts einschiebt
   (gleiche Breite wie die Sidebar), mit Header (Icon+Typ), Tabs falls nötig, und Footer-Buttons
   "Löschen" (links) / "Abbrechen" / "Fertig" (rechts, Go-Akzentfarbe).
9. **Statusleiste unten** (optional/Stretch) — dezente Leiste mit Verbindungsstatus, Node-Anzahl,
   Versionsinfo statt auffälligem Badge.
10. **Icons** — monochromes SVG-Icon-Set statt Emoji (Node-RED nutzt FontAwesome-artige Icons);
    Gopher-Bezüge nur im Logo/Branding, nicht als Spielerei in der Node-Palette.

---

## Farbpalette: Go statt Node-RED-Rot

Node-RED liefert nur das **Layout-/Interaktionsvorbild**. Alle Farbwerte kommen stattdessen aus
der offiziellen Go-Marken-Farbpalette (go.dev-Brand-Assets). Referenzwerte, die in Phase 0 gegen
die aktuellen offiziellen Go-Brand-Guidelines (go.dev) gegengeprüft werden sollen, bevor sie
final in `tailwind.config.js` landen:

| Rolle | Go-Farbe | Hex (verifizieren) | Verwendung im UI |
|---|---|---|---|
| Primär / Marke | Gopher Blue | `#00ADD8` | Header-Hintergrund, aktiver Deploy-Button, Fokus-/Selektions-Outline, Links |
| Primär dunkel | Deep Blue | `#007D9C` | Header-Hover/Active-States, dunklere Akzente |
| Sekundär | Aqua | `#00A29C` | Kategorie-Swatch, sekundäre Buttons |
| Sekundär | Fuchsia | `#CE3262` | Fehler-/Warn-Status, "Löschen"-Aktionen (statt NR-Rot) |
| Sekundär | Yellow | `#FDDD00` | "processing/busy"-Status, Hinweis-Badges |
| Neutral dunkel | Navy/Slate | `#17242D` | Text auf hellem Grund, evtl. Dark-Theme-Basis |
| Neutral hell | Light Blue-Gray | `#E0EBF5` | Canvas-/Palette-Hintergrund, Trennlinien |

Kategoriefarben der Node-Palette werden als Tinten/Schattierungen dieser Kernfarben abgeleitet
(z.B. input = Gopher Blue, output = Aqua, function = Deep Blue, storage = Navy-Tint, error/status
= Fuchsia), statt wie bei NR eine breite Regenbogenpalette pro Kategorie zu verwenden. Damit
bleibt das Ergebnis erkennbar "Go-farbig" statt NR-rot, auch wenn Struktur/Anordnung von NR
übernommen ist.

> **Hinweis:** Die obigen Hex-Werte sind der aktuell bekannte Go-Farbkanon, sollten aber zu
> Beginn von Phase 0 gegen die offiziellen go.dev-Brand-Assets gegengeprüft werden, statt
> ungeprüft übernommen zu werden — gleiches Vorgehen wie zuvor für die NR-Layoutwerte
> (Grid-Grau, Node-Geometrie, Typografie), die weiterhin aus einer laufenden Node-RED-Instanz
> bzw. deren Theme-CSS abgeleitet werden.

---

## Phasenplan

Jede Phase = ein eigener, review-barer PR. Reihenfolge so gewählt, dass jede Phase auf der
vorherigen aufbaut und das UI nach jeder Phase in einem funktionsfähigen Zustand bleibt.

### Phase 0 — Design Tokens & Grundlage
- Go-Brand-Farbwerte (siehe Tabelle oben) gegen go.dev-Brand-Assets verifizieren; Node-RED-Geometrie
  (Höhe/Breite/Radius/Port-Größe, Grid-Raster) weiterhin aus einer laufenden NR-Instanz/deren
  Theme-CSS ableiten. Beides zusammen als Tailwind-Theme-Erweiterung (`tailwind.config.js`) +
  CSS-Variablen in `web/src/styles/tailwind.css` ablegen (`--gr-header-bg`, `--gr-cat-input`,
  `--gr-cat-output`, `--gr-accent`, …). Präfix bewusst `gr-` (Go-RED) statt `nr-`, um klarzustellen,
  dass es sich um eigene Tokens handelt, nicht um 1:1 kopierte NR-Werte.
- Font-Stack auf Node-REDs UI-Font (Systemschrift-Stack, kompaktere Größen) umstellen.
- Kleine interne Style-Guide-Seite (Dev-only Route) zur visuellen Abnahme der Tokens (Farben +
  Geometrie gemeinsam).
- **Keine Interface-Änderungen**, rein CSS/Config.

### Phase 1 — App-Shell (Header, Deploy-Button, Flow-Tabs, Hauptmenü)
- Neue `Header.tsx`: Kopfleiste in Gopher Blue, Logo, Hamburger-Menü mit Save/Export/Import.
- `Toolbar.tsx` → wird zu schmaler Flow-Tab-Leiste (ersetzt Flow-Dropdown durch Tabs).
- Deploy-Button-Logik: aktiv/inaktiv abhängig von "dirty"-State des Flows (neuer, lokaler UI-State,
  kein Backend-Feld nötig).

### Phase 2 — Node-Palette
- `NodePalette.tsx` optisch auf NR-Layout + Go-Kategorie-Farben umstellen, Zeilenhöhe/Padding
  verdichten, Reihenfolge der Kategorien fixieren.
- Emoji-Icons durch SVG-Icon-Map ersetzen (kleine neue `iconMap.ts`, weiterhin DOMPurify für
  server-gelieferte Custom-Icons beibehalten).

### Phase 3 — Canvas & Node-Optik
- `FlowCanvas.tsx`: Punktraster, Zoom-Controls unten links, Edge-Styling (grau/blau bei Selektion).
- `NodeComponent.tsx`: Redesign auf durchgehende Farbbox, Ports als Kanten-Konnektoren,
  Status-Label unterhalb der Node, blaue gestrichelte Selektions-Outline.
- Betrifft auch `InjectNode.tsx`/`DebugNode.tsx` (spezialisierte Node-Renderer).

### Phase 4 — Rechte Sidebar mit Tabs (Info/Debug)
- Neue `SidebarTabs.tsx`-Hülle mit vertikalem Icon-Tab-Streifen.
- `Sidebar.tsx`-Inhalt wird "Info"-Tab, `MessageLogPanel.tsx`-Inhalt wird "Debug"-Tab.
- Ein-/Ausblendbarkeit über Icon-Streifen statt separatem Toggle-Button in der Toolbar.

### Phase 5 — Edit-Tray statt zentrierter Modals
- `NodeConfigModal.tsx` → seitlich einschiebendes Tray-Panel (rechts, Breite = Sidebar-Breite),
  Footer mit Löschen/Abbrechen/Fertig.
- `ExportModal.tsx`/`ImportModal.tsx` optisch an Tray-Look angleichen; ob sie ebenfalls zu
  Trays werden oder als Dialoge bleiben, ist eine offene Entscheidung (siehe Risiken) — höherer
  Aufwand, da neue Slide-in-Infrastruktur nötig ist.

### Phase 6 — Feinschliff
- `ToastNotification.tsx` an NR-Notification-Optik angleichen (weißer Kasten, farbiger linker
  Rand, Icon).
- `WebSocketStatus.tsx` in dezente Statusleiste unten überführen.
- Optional: Dark-Theme-Tokens (NR hat seit v3 ein offizielles Dark Theme) als Stretch-Goal.

---

## Nicht betroffen / bewusst außerhalb des Scopes

- Backend (`internal/*`, `cmd/*`), WebSocket-Protokoll, DTOs — keine Änderungen.
- Domain-Typen in `web/src/types/**` — falls doch touched (z.B. neues UI-only Feld wie "dirty"-Flag),
  gilt weiterhin die **Interface-Verifikation** aus `web/src/components/AGENTS.md`
  (go build/vet → go generate → tsc → npm test). Erwartung: nicht nötig, da rein visuell.
- Funktionale Erweiterungen (neue Node-Typen, neue Backend-Features) — separates Thema.

---

## Verifikation je Phase

- `cd web && npm run typecheck && npm run lint && npm test` nach jeder Phase.
- Manueller Smoke-Test im Browser (`npm run dev`): Flow anlegen, Node ziehen, verbinden,
  konfigurieren, deployen, Debug-Panel prüfen — gemäß Projektregel "UI-Änderungen im Browser
  testen, bevor sie als fertig gemeldet werden".
- Bestehende Komponenten-Tests (`web/src/test/*.test.tsx`) müssen an neue Klassennamen/Struktur
  angepasst werden, wo sie auf konkrete Tailwind-Klassen statt Rollen/Text prüfen.

---

## Offene Fragen / Risiken

1. **Zwei getrennte Quellen für Design-Tokens** vor Phase 0 verifizieren: (a) Go-Brand-Farbwerte
   gegen offizielle go.dev-Brand-Assets, (b) NR-Geometrie/Grid/Typografie gegen eine laufende
   Node-RED-Instanz bzw. deren Theme-CSS. Wird das vermischt oder ungeprüft übernommen, wirkt
   das Ergebnis weder "Go-artig" noch "Node-RED-artig", sondern beliebig.
2. **Edit-Tray (Phase 5)** ist der aufwändigste Schritt (neue Slide-in-Layout-Infrastruktur,
   Fokus-Trap, Responsiveness). Sollte separat freigegeben werden, bevor Aufwand investiert wird.
3. **Emoji → SVG-Icons**: Aufwand hängt davon ab, ob ein vorhandenes Icon-Set (z.B. Lucide/Feather,
   bereits MIT-lizenziert) genutzt werden kann, oder eigene Icons nötig sind.
4. Reihenfolge ist so gewählt, dass früh sichtbare Änderungen (Header, Palette, Canvas) vor den
   riskanteren/aufwändigeren Teilen (Tray) kommen — kann bei Bedarf umsortiert werden.
