### lys-shell Configuration - Hyprland Workspaces Module

## Hyprland Workspaces Module Configuration

settings-modules-hyprland-workspaces-min-workspace-count = Mindestanzahl Arbeitsflächen
    .description = Mindestanzahl angezeigter Schaltflächen (0 = nur aktive/belegte)

settings-modules-hyprland-workspaces-monitor-specific = Monitorspezifisch
    .description = Nur Arbeitsflächen auf diesem Monitor anzeigen

settings-modules-hyprland-workspaces-show-special = Spezielle anzeigen
    .description = Scratchpad-Arbeitsflächen einbeziehen

settings-modules-hyprland-workspaces-highlight-active-on-other-monitor = Aktive auf anderem Monitor hervorheben
    .description = Eine andere Farbe für aktive Arbeitsflächen auf anderen Monitoren verwenden (gilt nur, wenn { settings-modules-hyprland-workspaces-monitor-specific } aus ist)

settings-modules-hyprland-workspaces-urgent-show = Dringende anzeigen
    .description = Pulsanimation auf Arbeitsflächen mit dringenden Fenstern

settings-modules-hyprland-workspaces-display-mode = Anzeigemodus
    .description = Was jede Arbeitsfläche kennzeichnet (Beschriftung, Symbol oder nichts)

settings-modules-hyprland-workspaces-label-use-name = Namen verwenden
    .description = Namen der Arbeitsfläche statt Nummer anzeigen

settings-modules-hyprland-workspaces-numbering = Nummerierung
    .description = Wie Arbeitsflächennummern angezeigt werden (absolut oder relativ)

settings-modules-hyprland-workspaces-divider = Trenner
    .description = Text zwischen Arbeitsflächenkennung und App-Symbolen

settings-modules-hyprland-workspaces-app-icons-show = App-Symbole anzeigen
    .description = Fenstersymbole pro Arbeitsfläche anzeigen

settings-modules-hyprland-workspaces-app-icons-dedupe = Doppelte Symbole zusammenfassen
    .description = Ein Symbol pro Fensterklasse anzeigen

settings-modules-hyprland-workspaces-app-icons-fallback = Ersatzsymbol
    .description = Symbol für nicht erkannte Anwendungen

settings-modules-hyprland-workspaces-app-icons-empty = Symbol für leere
    .description = Platzhaltersymbol für leere Arbeitsflächen

settings-modules-hyprland-workspaces-icon-gap = Symbolabstand
    .description = Abstand zwischen App-Symbolen (rem)

settings-modules-hyprland-workspaces-workspace-padding = Innenabstand der Arbeitsfläche
    .description = Innenabstand für Inhalte, die breiter als der Kreis sind: App-Symbole, ein Trenner oder ein langer Name. Eine kurze Beschriftung erhält keinen: Sie bestimmt stattdessen die Kreisgröße

settings-modules-hyprland-workspaces-icon-size = Symbolgröße
    .description = Skalierungsfaktor für Arbeitsflächensymbole (0.25–3.0)

settings-modules-hyprland-workspaces-label-size = Beschriftungsgröße
    .description = Skalierungsfaktor für Arbeitsflächenbeschriftungen (0.25–3.0)

settings-modules-hyprland-workspaces-workspace-ignore = Arbeitsflächen ignorieren
    .description = Glob-Muster für auszublendende Arbeitsflächen-IDs

settings-modules-hyprland-workspaces-active-indicator = Aktivitätsanzeige
    .description = Visueller Stil der aktiven Arbeitsfläche

settings-modules-hyprland-workspaces-active-color = Aktive Farbe
    .description = Farbe für Symbole und Beschriftungen der fokussierten Arbeitsfläche

settings-modules-hyprland-workspaces-active-on-other-monitor-color = Farbe für aktive auf anderem Monitor
    .description = Farbe für Symbole und Beschriftungen fokussierter Arbeitsflächen, die auf anderen Monitoren aktiv sind (nur verwendet, wenn { settings-modules-hyprland-workspaces-highlight-active-on-other-monitor } an ist)

settings-modules-hyprland-workspaces-occupied-color = Farbe für belegte
    .description = Farbe für Symbole und Beschriftungen belegter Arbeitsflächen

settings-modules-hyprland-workspaces-empty-color = Farbe für leere
    .description = Farbe für Symbole und Beschriftungen leerer Arbeitsflächen

settings-modules-hyprland-workspaces-container-bg-color = Containerhintergrund
    .description = Hintergrundfarbe des Arbeitsflächencontainers

settings-modules-hyprland-workspaces-border-show = Rahmen anzeigen
    .description = Rahmen um den Arbeitsflächencontainer anzeigen

settings-modules-hyprland-workspaces-border-color = Rahmenfarbe
    .description = Rahmenfarbe des Arbeitsflächencontainers

settings-modules-hyprland-workspaces-workspace-map = Arbeitsflächenzuordnung
    .description = Symbol- und Farbüberschreibungen pro Arbeitsfläche

settings-modules-hyprland-workspaces-app-icon-map = App-Symbolzuordnung
    .description = Zuordnung von Fensterklasse/-titel zu Symbolen

settings-modules-hyprland-workspaces-urgent-mode = Dringlichkeitsmodus
    .description = Wie dringende Arbeitsflächen hervorgehoben werden (pro Arbeitsfläche oder pro Anwendung)


## DisplayMode variants
enum-display-mode-label = Beschriftung
enum-display-mode-icon = Symbol
enum-display-mode-none = Nichts

## ActiveIndicator variants
enum-active-indicator-background = Hintergrund
enum-active-indicator-underline = Unterstreichung

settings-modules-hyprland-workspaces-left-click = Linksklick
    .description = Aktion bei Linksklick
settings-modules-hyprland-workspaces-middle-click = Mittelklick
    .description = Aktion bei Mittelklick
settings-modules-hyprland-workspaces-right-click = Rechtsklick
    .description = Aktion bei Rechtsklick
settings-modules-hyprland-workspaces-scroll-up = Nach oben scrollen
    .description = Aktion beim Scrollen nach oben
settings-modules-hyprland-workspaces-scroll-down = Nach unten scrollen
    .description = Aktion beim Scrollen nach unten
