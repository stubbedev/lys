### lys-shell Configuration - Niri Workspaces Module

## Niri Workspaces Module Configuration

settings-modules-niri-workspaces-min-workspace-count = Mindestanzahl Arbeitsflächen
    .description = Numerisch benannte Arbeitsflächen bis zu diesem Index immer anzeigen, auch wenn sie leer sind

settings-modules-niri-workspaces-monitor-specific = Monitorspezifisch
    .description = Nur Arbeitsflächen auf diesem Monitor anzeigen

settings-modules-niri-workspaces-hide-trailing-empty = Leere am Ende ausblenden
    .description = Die von niri automatisch angelegte leere Arbeitsfläche am Ende jeder Ausgabe ausblenden

settings-modules-niri-workspaces-display-mode = Anzeigemodus
    .description = Was jede Arbeitsfläche kennzeichnet (Beschriftung, Symbol oder nichts)

settings-modules-niri-workspaces-label-strategy = Beschriftungsstrategie
    .description = Wie die Arbeitsflächenbeschriftung aus Name und Index gebildet wird

settings-modules-niri-workspaces-divider = Trenner
    .description = Text zwischen Arbeitsflächenkennung und App-Symbolen

settings-modules-niri-workspaces-urgent-show = Dringende anzeigen
    .description = Pulsanimation auf Arbeitsflächen mit dringenden Fenstern

settings-modules-niri-workspaces-urgent-mode = Dringlichkeitsmodus
    .description = Die ganze Arbeitsfläche oder nur das dringende App-Symbol pulsieren lassen

settings-modules-niri-workspaces-app-icons-show = App-Symbole anzeigen
    .description = Fenstersymbole pro Arbeitsfläche anzeigen

settings-modules-niri-workspaces-app-icons-dedupe = Doppelte Symbole zusammenfassen
    .description = Ein Symbol pro app_id statt eines pro Fenster anzeigen

settings-modules-niri-workspaces-app-icons-fallback = Ersatzsymbol
    .description = Symbol für Fenster, die nicht in app-icon-map erfasst sind

settings-modules-niri-workspaces-app-icons-empty = Symbol für leere
    .description = Symbol, wenn eine Arbeitsfläche keine Anwendungsfenster hat

settings-modules-niri-workspaces-icon-gap = Symbolabstand
    .description = Abstand zwischen App-Symbolen

settings-modules-niri-workspaces-workspace-padding = Innenabstand der Arbeitsfläche
    .description = Innenabstand für Inhalte, die breiter als der Kreis sind: App-Symbole, ein Trenner oder ein langer Name. Eine kurze Beschriftung erhält keinen: Sie bestimmt stattdessen die Kreisgröße

settings-modules-niri-workspaces-icon-size = Symbolgröße
    .description = Skalierungsfaktor für Arbeitsflächensymbole (0.25–3.0)

settings-modules-niri-workspaces-label-size = Beschriftungsgröße
    .description = Skalierungsfaktor für Arbeitsflächenbeschriftungen (0.25–3.0)

settings-modules-niri-workspaces-workspace-ignore = Arbeitsflächen ignorieren
    .description = Glob-Muster, die mit Name, Index oder ID abgeglichen werden

settings-modules-niri-workspaces-active-indicator = Aktivitätsanzeige
    .description = Visueller Stil der aktiven Arbeitsfläche

settings-modules-niri-workspaces-active-color = Aktive Farbe
    .description = Farbe für Symbol und Beschriftung der aktiven Arbeitsfläche

settings-modules-niri-workspaces-occupied-color = Farbe für belegte
    .description = Farbe für Symbole und Beschriftungen belegter Arbeitsflächen

settings-modules-niri-workspaces-empty-color = Farbe für leere
    .description = Farbe für Symbole und Beschriftungen leerer Arbeitsflächen

settings-modules-niri-workspaces-container-bg-color = Containerhintergrund
    .description = Hintergrundfarbe des Arbeitsflächencontainers

settings-modules-niri-workspaces-border-show = Rahmen anzeigen
    .description = Rahmen um den Arbeitsflächencontainer anzeigen

settings-modules-niri-workspaces-border-color = Rahmenfarbe
    .description = Rahmenfarbe des Arbeitsflächencontainers

settings-modules-niri-workspaces-workspace-map = Arbeitsflächenzuordnung
    .description = Symbol- und Farbüberschreibungen pro Arbeitsfläche, nach Name oder ID

settings-modules-niri-workspaces-app-icon-map = App-Symbolzuordnung
    .description = Zuordnung von Fenster-app_id oder -Titel zu Symbolen

settings-modules-niri-workspaces-left-click = Linksklick
    .description = Aktion bei Linksklick

settings-modules-niri-workspaces-middle-click = Mittelklick
    .description = Aktion bei Mittelklick

settings-modules-niri-workspaces-right-click = Rechtsklick
    .description = Aktion bei Rechtsklick

settings-modules-niri-workspaces-scroll-up = Nach oben scrollen
    .description = Aktion beim Scrollen nach oben

settings-modules-niri-workspaces-scroll-down = Nach unten scrollen
    .description = Aktion beim Scrollen nach unten


## LabelStrategy variants
enum-label-strategy-index = Index
enum-label-strategy-name-or-index = Name oder Index
enum-label-strategy-name-only = Nur Name
enum-label-strategy-index-and-name = Index und Name
