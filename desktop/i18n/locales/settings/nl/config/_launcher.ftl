settings-launcher-location = Positie
    .description = Waar de starter op het scherm verschijnt

settings-launcher-width = Breedte
    .description = Breedte van het oppervlak — een factor van de standaard (1.0) of pixels (bijv. "800px")

settings-launcher-lines = Regels
    .description = Zichtbare resultaatregels

settings-launcher-monitor = Monitor
    .description = Uitvoerconnector om op te tonen (leeg = uitvoer met focus)

settings-launcher-modes = Modi
    .description = Ingeschakelde modi, in tabvolgorde (drun, run, window, ssh, filebrowser, combi, keys, of een scriptnaam)

settings-launcher-cycle = Selectie rondgaan
    .description = De selectie aan de randen van de lijst laten doorlopen

settings-launcher-matching = Zoekmethode
    .description = Hoe getypte tekst met items wordt vergeleken

settings-launcher-tokenize = Opsplitsen
    .description = Elk woord van de zoekopdracht afzonderlijk vergelijken

settings-launcher-negate-char = Negatieteken
    .description = Voorvoegsel van een woord dat overeenkomsten uitsluit (rofi -matching-negate-char)

settings-launcher-normalize-match = Normaliseren bij zoeken
    .description = Accenten en Unicode-varianten negeren bij het zoeken

settings-launcher-sort = Resultaten sorteren
    .description = Resultaten rangschikken op overeenkomst in plaats van lijstvolgorde

settings-launcher-sorting-method = Sorteermethode
    .description = Rangschikkingsalgoritme dat wordt gebruikt als sorteren is ingeschakeld

settings-launcher-case = Hoofdlettergevoeligheid
    .description = Hoe hoofdletters het zoeken beïnvloeden

settings-launcher-terminal = Terminal
    .description = Terminalemulator voor terminaltoepassingen (leeg = automatisch detecteren)

settings-launcher-show-icons = Pictogrammen tonen
    .description = Pictogrammen naast items tonen

settings-launcher-icon-theme = Pictogramthema
    .description = Overschrijving van het pictogramthema (leeg = systeemthema)

settings-launcher-sidebar-mode = Zijbalkmodus
    .description = Modustabbladen onderaan de starter tonen

settings-launcher-auto-select = Automatisch selecteren
    .description = Automatisch accepteren als er precies één resultaat over is

settings-launcher-hover-select = Selecteren bij zweven
    .description = De rij onder de muisaanwijzer selecteren

settings-launcher-fixed-num-lines = Vaste hoogte
    .description = De lijsthoogte vast houden op het ingestelde aantal regels

settings-launcher-display-names = Weergavenamen van modi
    .description = Overschrijvingen van de weergavenaam per modus (bijv. drun = "apps")

settings-launcher-scripts = Scriptmodi
    .description = Aangepaste scriptmodi: naam → uitvoerbaar bestand (rofi-scriptprotocol)

settings-launcher-keybindings = Sneltoetsen
    .description = Overschrijvingen van sneltoetsen: actie → door komma's gescheiden toetsen (niet-ingestelde acties behouden de rofi-standaarden)

settings-launcher-mouse-bindings = Muisbindingen
    .description = Overschrijvingen van muisbindingen: actie (me-* knoppen, ml-* scrollen) → door komma's gescheiden knoppen (bijv. "MouseDPrimary", "ScrollDown")

settings-launcher-font = Lettertype
    .description = Lettertype van de starter als Pango-beschrijving (bijv. "Inter 12"); leeg behoudt het shell-lettertype

settings-launcher-styles = Stijlvoorinstellingen
    .description = Benoemde stijlen die per aanroep te kiezen zijn met -style <name>: naam → de CSS die voor die sessie wordt toegepast

settings-launcher-preview-cmd = Voorbeeldopdracht
    .description = Opdracht die een miniatuur voor een rij maakt: {"{"}input{"}"} het bestand, {"{"}output{"}"} de te schrijven afbeelding, {"{"}size{"}"} de grootte in pixels; leeg gebruikt de miniatuurgeneratoren van het systeem

settings-launcher-history = Geschiedenis
    .description = Startgeschiedenis en rangschikking op frecency

settings-launcher-drun = Toepassingen
    .description = drun-modus (toepassingen)

settings-launcher-run = Opdrachten
    .description = run-modus (opdrachten)

settings-launcher-window = Vensters
    .description = Vensterwisselmodus

settings-launcher-ssh = SSH
    .description = SSH-hostmodus

settings-launcher-filebrowser = Bestandsbrowser
    .description = Bestandsbrowsermodus

settings-launcher-combi = Combi
    .description = Modus met gecombineerde modi

settings-launcher-history-enable = Geschiedenis inschakelen
    .description = Starts vastleggen en veelgebruikte items als eerste rangschikken

settings-launcher-history-max-size = Geschiedenisgrootte
    .description = Maximaal aantal onthouden items per modus

settings-launcher-drun-categories = Categorieën
    .description = Alleen toepassingen binnen deze categorieën tonen (leeg = alle)

settings-launcher-drun-exclude-categories = Uitgesloten categorieën
    .description = Toepassingen binnen deze categorieën verbergen

settings-launcher-drun-match-fields = Zoekvelden
    .description = Velden van de desktop-entry die tijdens het typen worden doorzocht

settings-launcher-drun-display-format = Weergaveformaat
    .description = Rijsjabloon: {"{name}"}, {"{generic}"}, {"{exec}"}, {"{categories}"}, {"{comment}"}; [..] wordt alleen getoond als het gevuld is

settings-launcher-drun-show-actions = Acties tonen
    .description = Acties uit desktopbestanden als extra rijen tonen

settings-launcher-drun-url-launcher = URL-starter
    .description = Opdracht die desktop-entries met Type=Link opent (leeg = de ingebouwde opener)

settings-launcher-run-run-command = Uitvoeropdracht
    .description = Gewoon accepteersjabloon ({"{cmd}"})

settings-launcher-run-shell-command = Terminalopdracht
    .description = Sjabloon voor uitvoeren in terminal ({"{terminal}"}, {"{cmd}"})

settings-launcher-run-list-command = Lijstopdracht
    .description = Extra opdracht waarvan de stdout-regels items toevoegen

settings-launcher-window-format = Weergaveformaat
    .description = Rijsjabloon: {"{w}"} werkblad, {"{c}"} klasse, {"{t}"} titel, {"{n}"} naam, {"{r}"} rol

settings-launcher-window-match-fields = Zoekvelden
    .description = Venstervelden die tijdens het typen worden doorzocht

settings-launcher-window-hide-active = Actief venster verbergen
    .description = Het venster met focus uit de lijst verbergen

settings-launcher-window-close-on-delete = Sluiten bij Delete
    .description = Shift-Delete sluit het geselecteerde venster

settings-launcher-ssh-client = SSH-client
    .description = Uitvoerbaar bestand van de SSH-client

settings-launcher-ssh-command = Verbindingsopdracht
    .description = Verbindingssjabloon ({"{terminal}"}, {"{ssh-client}"}, {"{host}"})

settings-launcher-ssh-parse-hosts = /etc/hosts inlezen
    .description = Hosts uit /etc/hosts opnemen

settings-launcher-ssh-parse-known-hosts = known_hosts inlezen
    .description = Hosts uit ~/.ssh/known_hosts opnemen

settings-launcher-filebrowser-directory = Startmap
    .description = Map waarin de browser opent (leeg = persoonlijke map)

settings-launcher-filebrowser-sorting-method = Sortering
    .description = Volgorde van bestanden

settings-launcher-filebrowser-directories-first = Mappen eerst
    .description = Mappen vóór bestanden tonen

settings-launcher-filebrowser-show-hidden = Verborgen tonen
    .description = Verborgen bestanden tonen

settings-launcher-filebrowser-command = Openopdracht
    .description = Opdracht die het gekozen bestand opent (leeg = de ingebouwde opener)

settings-launcher-combi-modes = Gecombineerde modi
    .description = Modi die in de gecombineerde lijst worden samengevoegd

settings-launcher-combi-display-format = Weergaveformaat
    .description = Rijsjabloon ({"{mode}"}, {"{text}"})

enum-launcher-location-center = Midden
enum-launcher-location-north-west = Linksboven
enum-launcher-location-north = Boven
enum-launcher-location-north-east = Rechtsboven
enum-launcher-location-east = Rechts
enum-launcher-location-south-east = Rechtsonder
enum-launcher-location-south = Onder
enum-launcher-location-south-west = Linksonder
enum-launcher-location-west = Links

enum-launcher-matching-normal = Normaal
enum-launcher-matching-regex = Regex
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Fuzzy
enum-launcher-matching-prefix = Voorvoegsel

enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

enum-launcher-case-insensitive = Ongevoelig
enum-launcher-case-smart = Slim
enum-launcher-case-sensitive = Gevoelig

enum-launcher-drun-field-name = Naam
enum-launcher-drun-field-generic = Algemene naam
enum-launcher-drun-field-exec = Opdracht
enum-launcher-drun-field-categories = Categorieën
enum-launcher-drun-field-comment = Opmerking
enum-launcher-drun-field-keywords = Trefwoorden

enum-launcher-window-field-title = Titel
enum-launcher-window-field-class = Klasse
enum-launcher-window-field-name = Naam
enum-launcher-window-field-role = Rol
enum-launcher-window-field-desktop = Werkblad

enum-launcher-file-sort-name = Naam
enum-launcher-file-sort-mtime = Gewijzigd
enum-launcher-file-sort-atime = Geopend
enum-launcher-file-sort-ctime = Aangemaakt

settings-animations-launcher = Starter
    .description = Overschrijving van de verschijn-/verdwijnanimatie voor de toepassingsstarter

settings-launcher-search = Zoekpaneel
    .description = Het paneel dat `lys-shell launcher` zonder argumenten opent

settings-launcher-search-providers = Bronnen
    .description = Wat de lijst doorzoekt

settings-launcher-search-web-engine = Naam webzoekmachine
    .description = De naam van de zoekmachine op de terugvalrij

settings-launcher-search-web-url = URL webzoekmachine
    .description = De URL, met {"{"}query{"}"} waar de zoekopdracht komt

settings-launcher-search-file-roots = Mappen bestandsindex
    .description = Mappen die de bestandsindex doorloopt; leeg indexeert de persoonlijke map

settings-launcher-search-file-exclude = Uitsluitingen bestandsindex
    .description = Mapnamen die de bestandsindex overslaat

settings-launcher-search-commands-dir = Map scriptopdrachten
    .description = Leeg gebruikt launcher/commands in de configuratiemap

settings-launcher-search-currency-rates = Wisselkoersen
    .description = De dagelijkse referentiekoersen van de Europese Centrale Bank ophalen, hooguit één keer per dag, voor valuta-antwoorden

settings-launcher-search-previews = Voorbeelden
    .description = Het zijpaneel tonen voor bestanden, fragmenten, vensters en klemborditems, in het zoekpaneel en de venster- en klembordmodi

settings-launcher-search-quicklinks = Snelkoppelingen
    .description = Links om op naam of trefwoord te openen

settings-launcher-search-snippets = Fragmenten
    .description = Tekst om op naam of trefwoord te kopiëren

enum-launcher-search-provider-apps = Toepassingen
enum-launcher-search-provider-windows = Geopende vensters
enum-launcher-search-provider-files = Bestanden
enum-launcher-search-provider-calculator = Rekenmachine
enum-launcher-search-provider-system = Systeemopdrachten
enum-launcher-search-provider-settings = Instellingenpagina's
enum-launcher-search-provider-clipboard = Klembordgeschiedenis
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Unicode-tekens
enum-launcher-search-provider-vault = Bitwarden-kluis
enum-launcher-search-provider-links = Snelkoppelingen en fragmenten
enum-launcher-search-provider-browser-bookmarks = Browserbladwijzers
enum-launcher-search-provider-browser-history = Browsergeschiedenis
enum-launcher-search-provider-quick-toggles = Snelle schakelaars
enum-launcher-search-provider-media = Mediabediening
enum-launcher-search-provider-capture = Schermafdrukken en opnemen
enum-launcher-search-provider-processes = Proces beëindigen
enum-launcher-search-provider-scripts = Scriptopdrachten
enum-launcher-search-provider-shell = Shell-opdrachten
enum-launcher-search-provider-web = Op het web zoeken

settings-launcher-drun-display-format-placeholder-name = Toepassingsnaam

settings-launcher-drun-display-format-placeholder-generic = Algemene naam (bijv. Webbrowser)

settings-launcher-drun-display-format-placeholder-exec = Opdracht die het item uitvoert

settings-launcher-drun-display-format-placeholder-categories = Categorieën van de desktop-entry

settings-launcher-drun-display-format-placeholder-comment = Opmerking van de desktop-entry

settings-launcher-window-format-placeholder-w = Werkblad

settings-launcher-window-format-placeholder-c = Vensterklasse

settings-launcher-window-format-placeholder-t = Venstertitel

settings-launcher-window-format-placeholder-n = Vensternaam (de klasse ervan)

settings-launcher-window-format-placeholder-r = Vensterrol (altijd leeg)

settings-launcher-combi-display-format-placeholder-mode = Modus waar de rij vandaan komt

settings-launcher-combi-display-format-placeholder-text = De eigen tekst van de rij

settings-launcher-emoji = Emoji
    .description = Emoji in het zoekpaneel, het emojiraster en de emojimodus

settings-launcher-emoji-skin-tone = Huidskleur
    .description = De huidskleur van persoon- en handemoji; er een kiezen uit de acties van een emoji maakt die de standaard

enum-launcher-skin-tone-none = Standaard (geel)
enum-launcher-skin-tone-light = Licht
enum-launcher-skin-tone-medium-light = Middellicht
enum-launcher-skin-tone-medium = Getint
enum-launcher-skin-tone-medium-dark = Middeldonker
enum-launcher-skin-tone-dark = Donker

settings-launcher-run-run-command-placeholder-cmd = De getypte opdracht
settings-launcher-run-shell-command-placeholder-terminal = De terminal
settings-launcher-run-shell-command-placeholder-cmd = De getypte opdracht
settings-launcher-ssh-command-placeholder-terminal = De terminal
settings-launcher-ssh-command-placeholder-ssh-client = De SSH-client
settings-launcher-ssh-command-placeholder-host = De host
settings-launcher-preview-cmd-placeholder-input = Het bestand om te bekijken
settings-launcher-preview-cmd-placeholder-output = Waar de miniatuur heen gaat
settings-launcher-preview-cmd-placeholder-size = De grootte van de miniatuur in pixels

## Quick links and snippets

settings-launcher-quicklink-name = Naam
settings-launcher-quicklink-url = Adres
settings-launcher-quicklink-keyword = Trefwoord
settings-launcher-quicklink-icon = Pictogram
settings-launcher-quicklink-url-placeholder-query = De woorden getypt na het trefwoord
settings-launcher-snippet-name = Naam
settings-launcher-snippet-text = Tekst
settings-launcher-snippet-keyword = Trefwoord
