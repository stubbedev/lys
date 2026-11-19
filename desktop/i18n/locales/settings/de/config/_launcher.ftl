# Launcher (rofi replacement)

settings-launcher-location = Position
    .description = Wo der Starter auf dem Bildschirm erscheint

settings-launcher-width = Breite
    .description = Breite der Oberfläche – ein Vielfaches der Standardbreite (1.0) oder Pixel (z. B. "800px")

settings-launcher-lines = Zeilen
    .description = Sichtbare Ergebniszeilen

settings-launcher-monitor = Monitor
    .description = Ausgabeanschluss für die Anzeige (leer = fokussierte Ausgabe)

settings-launcher-modes = Modi
    .description = Aktivierte Modi in Tab-Reihenfolge (drun, run, window, ssh, filebrowser, combi, keys oder ein Skriptname)

settings-launcher-cycle = Auswahl umlaufen
    .description = Die Auswahl an den Listenenden umlaufen lassen

settings-launcher-matching = Abgleichmethode
    .description = Wie eingegebener Text mit Einträgen abgeglichen wird

settings-launcher-tokenize = In Wörter zerlegen
    .description = Jedes Wort der Suche unabhängig abgleichen

settings-launcher-negate-char = Negationszeichen
    .description = Wortpräfix, das Treffer ausschließt (rofi -matching-negate-char)

settings-launcher-normalize-match = Abgleich normalisieren
    .description = Akzente und Unicode-Varianten beim Abgleich ignorieren

settings-launcher-sort = Ergebnisse sortieren
    .description = Ergebnisse nach Trefferqualität statt Listenreihenfolge ordnen

settings-launcher-sorting-method = Sortiermethode
    .description = Ranking-Algorithmus bei aktivierter Sortierung

settings-launcher-case = Groß-/Kleinschreibung
    .description = Wie die Groß-/Kleinschreibung den Abgleich beeinflusst

settings-launcher-terminal = Terminal
    .description = Terminalemulator für Terminal-Apps (leer = automatisch erkennen)

settings-launcher-show-icons = Symbole anzeigen
    .description = Symbole neben den Einträgen anzeigen

settings-launcher-icon-theme = Symbolthema
    .description = Symbolthema überschreiben (leer = Systemthema)

settings-launcher-sidebar-mode = Seitenleistenmodus
    .description = Modus-Reiter am unteren Rand des Starters anzeigen

settings-launcher-auto-select = Automatisch auswählen
    .description = Automatisch übernehmen, wenn genau ein Ergebnis übrig ist

settings-launcher-hover-select = Auswahl beim Überfahren
    .description = Die Zeile unter dem Mauszeiger auswählen

settings-launcher-fixed-num-lines = Feste Höhe
    .description = Listenhöhe auf die eingestellte Zeilenzahl festlegen

settings-launcher-display-names = Anzeigenamen der Modi
    .description = Anzeigenamen pro Modus überschreiben (z. B. drun = "apps")

settings-launcher-scripts = Skriptmodi
    .description = Eigene Skriptmodi: Name → ausführbare Datei (rofi-Skriptprotokoll)

settings-launcher-keybindings = Tastenkürzel
    .description = Tastenkürzel überschreiben: Aktion → kommagetrennte Tasten (nicht gesetzte Aktionen behalten die rofi-Standards)

settings-launcher-mouse-bindings = Maustasten
    .description = Mausbelegungen überschreiben: Aktion (me-* Tasten, ml-* Scrollen) → kommagetrennte Tasten (z. B. "MouseDPrimary", "ScrollDown")

settings-launcher-font = Schrift
    .description = Schrift des Starters als Pango-Beschreibung (z. B. "Inter 12"); leer behält die Shell-Schrift

settings-launcher-styles = Stilvorlagen
    .description = Benannte Erscheinungsbilder, pro Aufruf mit -style <name> wählbar: Name → das CSS für diese Sitzung

settings-launcher-preview-cmd = Vorschaubefehl
    .description = Befehl, der ein Zeilen-Vorschaubild erzeugt: {"{"}input{"}"} die Datei, {"{"}output{"}"} das zu schreibende Bild, {"{"}size{"}"} die Pixelgröße; leer verwendet die Vorschaubildgeneratoren des Systems

settings-launcher-history = Verlauf
    .description = Startverlauf und Frecency-Ranking

settings-launcher-drun = Anwendungen
    .description = drun-Modus (Anwendungen)

settings-launcher-run = Befehle
    .description = run-Modus (Befehle)

settings-launcher-window = Fenster
    .description = Fensterwechsel-Modus

settings-launcher-ssh = SSH
    .description = SSH-Host-Modus

settings-launcher-filebrowser = Dateibrowser
    .description = Dateibrowser-Modus

settings-launcher-combi = Kombiniert
    .description = Modus mit kombinierten Modi

settings-launcher-history-enable = Verlauf aktivieren
    .description = Starts aufzeichnen und häufig genutzte Einträge zuerst einordnen

settings-launcher-history-max-size = Verlaufsgröße
    .description = Maximal gemerkte Einträge pro Modus

settings-launcher-drun-categories = Kategorien
    .description = Nur Anwendungen aus diesen Kategorien anzeigen (leer = alle)

settings-launcher-drun-exclude-categories = Ausgeschlossene Kategorien
    .description = Anwendungen aus diesen Kategorien ausblenden

settings-launcher-drun-match-fields = Abgleichfelder
    .description = Felder des Desktop-Eintrags, die beim Tippen durchsucht werden

settings-launcher-drun-display-format = Anzeigeformat
    .description = Zeilenvorlage: {"{name}"}, {"{generic}"}, {"{exec}"}, {"{categories}"}, {"{comment}"}; [..] wird nur angezeigt, wenn gefüllt

settings-launcher-drun-show-actions = Aktionen anzeigen
    .description = Aktionen aus Desktop-Dateien als zusätzliche Zeilen anzeigen

settings-launcher-drun-url-launcher = URL-Starter
    .description = Befehl zum Öffnen von Desktop-Einträgen mit Type=Link (leer = eingebauter Öffner)

settings-launcher-run-run-command = Ausführungsbefehl
    .description = Einfache Vorlage zum Übernehmen ({"{cmd}"})

settings-launcher-run-shell-command = Terminalbefehl
    .description = Vorlage für die Ausführung im Terminal ({"{terminal}"}, {"{cmd}"})

settings-launcher-run-list-command = Listenbefehl
    .description = Zusätzlicher Befehl, dessen Ausgabezeilen Einträge hinzufügen

settings-launcher-window-format = Anzeigeformat
    .description = Zeilenvorlage: {"{w}"} Arbeitsfläche, {"{c}"} Klasse, {"{t}"} Titel, {"{n}"} Name, {"{r}"} Rolle

settings-launcher-window-match-fields = Abgleichfelder
    .description = Fensterfelder, die beim Tippen durchsucht werden

settings-launcher-window-hide-active = Aktives Fenster ausblenden
    .description = Das aktuell fokussierte Fenster nicht in der Liste zeigen

settings-launcher-window-close-on-delete = Beim Löschen schließen
    .description = Umschalt+Entf schließt das ausgewählte Fenster

settings-launcher-ssh-client = SSH-Client
    .description = Programmdatei des SSH-Clients

settings-launcher-ssh-command = Verbindungsbefehl
    .description = Verbindungsvorlage ({"{terminal}"}, {"{ssh-client}"}, {"{host}"})

settings-launcher-ssh-parse-hosts = /etc/hosts auswerten
    .description = Hosts aus /etc/hosts einbeziehen

settings-launcher-ssh-parse-known-hosts = known_hosts auswerten
    .description = Hosts aus ~/.ssh/known_hosts einbeziehen

settings-launcher-filebrowser-directory = Startordner
    .description = Ordner, in dem der Browser öffnet (leer = Persönlicher Ordner)

settings-launcher-filebrowser-sorting-method = Sortierung
    .description = Reihenfolge der Dateien

settings-launcher-filebrowser-directories-first = Ordner zuerst
    .description = Ordner vor Dateien auflisten

settings-launcher-filebrowser-show-hidden = Verborgene anzeigen
    .description = Verborgene Dateien anzeigen

settings-launcher-filebrowser-command = Öffnungsbefehl
    .description = Befehl zum Öffnen der gewählten Datei (leer = eingebauter Öffner)

settings-launcher-combi-modes = Kombinierte Modi
    .description = Modi, die in die kombinierte Liste einfließen

settings-launcher-combi-display-format = Anzeigeformat
    .description = Zeilenvorlage ({"{mode}"}, {"{text}"})

## LauncherLocation variants
enum-launcher-location-center = Mitte
enum-launcher-location-north-west = Oben links
enum-launcher-location-north = Oben
enum-launcher-location-north-east = Oben rechts
enum-launcher-location-east = Rechts
enum-launcher-location-south-east = Unten rechts
enum-launcher-location-south = Unten
enum-launcher-location-south-west = Unten links
enum-launcher-location-west = Links

## LauncherMatching variants
enum-launcher-matching-normal = Normal
enum-launcher-matching-regex = Regex
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Unscharf
enum-launcher-matching-prefix = Präfix

## LauncherSorting variants
enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

## LauncherCase variants
enum-launcher-case-insensitive = Ignorieren
enum-launcher-case-smart = Intelligent
enum-launcher-case-sensitive = Beachten

## LauncherDrunField variants
enum-launcher-drun-field-name = Name
enum-launcher-drun-field-generic = Allgemeiner Name
enum-launcher-drun-field-exec = Befehl
enum-launcher-drun-field-categories = Kategorien
enum-launcher-drun-field-comment = Kommentar
enum-launcher-drun-field-keywords = Schlüsselwörter

## LauncherWindowField variants
enum-launcher-window-field-title = Titel
enum-launcher-window-field-class = Klasse
enum-launcher-window-field-name = Name
enum-launcher-window-field-role = Rolle
enum-launcher-window-field-desktop = Arbeitsfläche

## LauncherFileSort variants
enum-launcher-file-sort-name = Name
enum-launcher-file-sort-mtime = Geändert
enum-launcher-file-sort-atime = Zugegriffen
enum-launcher-file-sort-ctime = Erstellt

## Animations
settings-animations-launcher = Starter
    .description = Ein-/Ausblendanimation des Anwendungsstarters überschreiben

# Search panel

settings-launcher-search = Suchfeld
    .description = Das Feld, das `lys-shell launcher` ohne Argumente öffnet

settings-launcher-search-providers = Quellen
    .description = Was die Liste durchsucht

settings-launcher-search-web-engine = Name der Websuche
    .description = Name der Suchmaschine in der Ersatzzeile

settings-launcher-search-web-url = URL der Websuche
    .description = Die URL, mit {"{"}query{"}"} an der Stelle des Suchbegriffs

settings-launcher-search-file-roots = Ordner des Dateiindex
    .description = Ordner, die der Dateiindex durchläuft; leer indiziert den persönlichen Ordner

settings-launcher-search-file-exclude = Ausnahmen des Dateiindex
    .description = Ordnernamen, die der Dateiindex überspringt

settings-launcher-search-commands-dir = Ordner für Skriptbefehle
    .description = Leer verwendet launcher/commands im Konfigurationsordner

settings-launcher-search-currency-rates = Wechselkurse
    .description = Die täglichen Referenzkurse der Europäischen Zentralbank höchstens einmal täglich für Währungsantworten abrufen

settings-launcher-search-previews = Vorschauen
    .description = Seitenbereich für Dateien, Textbausteine, Fenster und Einträge der Zwischenablage im Suchfeld sowie in den Fenster- und Zwischenablage-Modi anzeigen

settings-launcher-search-quicklinks = Schnelllinks
    .description = Links, die nach Name oder Schlüsselwort geöffnet werden

settings-launcher-search-snippets = Textbausteine
    .description = Text, der nach Name oder Schlüsselwort kopiert wird

enum-launcher-search-provider-apps = Anwendungen
enum-launcher-search-provider-windows = Offene Fenster
enum-launcher-search-provider-files = Dateien
enum-launcher-search-provider-calculator = Rechner
enum-launcher-search-provider-system = Systembefehle
enum-launcher-search-provider-settings = Einstellungsseiten
enum-launcher-search-provider-clipboard = Verlauf der Zwischenablage
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Unicode-Zeichen
enum-launcher-search-provider-vault = Bitwarden-Tresor
enum-launcher-search-provider-links = Schnelllinks und Textbausteine
enum-launcher-search-provider-browser-bookmarks = Browser-Lesezeichen
enum-launcher-search-provider-browser-history = Browserverlauf
enum-launcher-search-provider-quick-toggles = Schnellschalter
enum-launcher-search-provider-media = Mediensteuerung
enum-launcher-search-provider-capture = Bildschirmfotos und Aufnahme
enum-launcher-search-provider-processes = Prozess beenden
enum-launcher-search-provider-scripts = Skriptbefehle
enum-launcher-search-provider-shell = Shell-Befehle
enum-launcher-search-provider-web = Websuche

## Format placeholders

settings-launcher-drun-display-format-placeholder-name = Name der Anwendung

settings-launcher-drun-display-format-placeholder-generic = Allgemeiner Name (z. B. Webbrowser)

settings-launcher-drun-display-format-placeholder-exec = Befehl, den der Eintrag ausführt

settings-launcher-drun-display-format-placeholder-categories = Kategorien des Desktop-Eintrags

settings-launcher-drun-display-format-placeholder-comment = Kommentar des Desktop-Eintrags

settings-launcher-window-format-placeholder-w = Arbeitsfläche

settings-launcher-window-format-placeholder-c = Fensterklasse

settings-launcher-window-format-placeholder-t = Fenstertitel

settings-launcher-window-format-placeholder-n = Fenstername (seine Klasse)

settings-launcher-window-format-placeholder-r = Fensterrolle (immer leer)

settings-launcher-combi-display-format-placeholder-mode = Modus, aus dem die Zeile stammt

settings-launcher-combi-display-format-placeholder-text = Der eigene Text der Zeile

# Emoji

settings-launcher-emoji = Emoji
    .description = Emoji im Suchfeld, im Emoji-Raster und im Emoji-Modus

settings-launcher-emoji-skin-tone = Hautfarbe
    .description = Die Hautfarbe von Personen- und Hand-Emoji; die Auswahl in den Aktionen eines Emoji macht sie zum Standard

## LauncherSkinTone variants
enum-launcher-skin-tone-none = Standard (Gelb)
enum-launcher-skin-tone-light = Hell
enum-launcher-skin-tone-medium-light = Mittelhell
enum-launcher-skin-tone-medium = Mittel
enum-launcher-skin-tone-medium-dark = Mitteldunkel
enum-launcher-skin-tone-dark = Dunkel

settings-launcher-run-run-command-placeholder-cmd = Der eingegebene Befehl
settings-launcher-run-shell-command-placeholder-terminal = Das Terminal
settings-launcher-run-shell-command-placeholder-cmd = Der eingegebene Befehl
settings-launcher-ssh-command-placeholder-terminal = Das Terminal
settings-launcher-ssh-command-placeholder-ssh-client = Der SSH-Client
settings-launcher-ssh-command-placeholder-host = Der Host
settings-launcher-preview-cmd-placeholder-input = Die Datei für die Vorschau
settings-launcher-preview-cmd-placeholder-output = Wohin das Vorschaubild geschrieben wird
settings-launcher-preview-cmd-placeholder-size = Die Größe des Vorschaubilds in Pixeln

## Quick links and snippets

settings-launcher-quicklink-name = Name
settings-launcher-quicklink-url = Adresse
settings-launcher-quicklink-keyword = Stichwort
settings-launcher-quicklink-icon = Symbol
settings-launcher-quicklink-url-placeholder-query = Die nach dem Stichwort getippten Wörter
settings-launcher-snippet-name = Name
settings-launcher-snippet-text = Text
settings-launcher-snippet-keyword = Stichwort
