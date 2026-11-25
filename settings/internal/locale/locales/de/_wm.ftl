### lys-settings: the lys-wm pages

settings-nav-wm-windows = Fenster
settings-page-wm-windows = Fenster
settings-nav-wm-effects = Schatten & Unschärfe
settings-page-wm-effects = Schatten & Unschärfe
settings-nav-wm-tiling = Kacheln & Reiter
settings-page-wm-tiling = Kacheln & Reiter
settings-nav-wm-binds = Tastenkürzel
settings-page-wm-binds = Tastenkürzel
settings-nav-wm-outputs = Bildschirme
settings-page-wm-outputs = Bildschirme
settings-nav-wm-session = Regeln & Sitzung
settings-page-wm-session = Regeln & Sitzung

wm-section-gaps = Abstände
    .description = Raum um und zwischen gekachelten Fenstern
wm-section-borders = Rahmen
    .description = Die Linie um jedes Fenster
wm-section-shape = Form & Transparenz
    .description = Eckenabrundung und Deckkraft der Fenster je nach Fokus
wm-section-shadow = Schatten
    .description = Der Schatten unter Fenstern
wm-section-blur = Unschärfe
    .description = Was durch durchscheinende Fenster sichtbar ist
wm-section-overview = Übersicht
    .description = Die Ansicht aller Arbeitsflächen auf einmal
wm-section-drag = Ziehvorschau
    .description = Das Fenster, das beim Ziehen dem Zeiger folgt
wm-section-autotile = Automatisches Kacheln
    .description = Wo sich ein neues Fenster in einer gekachelten Anordnung öffnet
wm-section-tabs = Reiter
    .description = Die Leiste über Reitergruppen
wm-section-keyboard = Tastatur
    .description = Belegung und Tastenwiederholung
wm-section-pointer = Maus
    .description = Mäuse und Trackballs
wm-section-touchpad = Touchpad
    .description = Tippen, Scrollen und Geschwindigkeit
wm-section-trackpoint = Trackpoint
    .description = Pointing-Sticks in der Tastatur eines Laptops
wm-section-binds = Tasten & Gesten
    .description = Tasten- und Mauskombinationen und die Aktionen, die sie ausführen
wm-section-outputs = Ausgaben
    .description = Modus, Skalierung, Position und Farbe jedes Bildschirms
wm-section-display = Bildschirm
    .description = Einstellungen, die für alle Bildschirme gelten
wm-section-rules = Fensterregeln
    .description = Wie passende Fenster sich öffnen und aussehen
wm-section-layer-rules = Leisten & Overlays
    .description = Wie Leiste, Dock, Pop-ups und andere Flächen der Shell aussehen
wm-rules-namespace = Fläche
wm-rules-blur-alpha-threshold = Alpha-Schwelle der Unschärfe
wm-layer-lys-shell-bar = Leiste
wm-layer-lys-shell-dock = Dock
wm-layer-lys-shell-wallpaper = Hintergrundbild
wm-layer-lys-shell-launcher = Starter
wm-layer-lys-shell-notification-popup = Benachrichtigungs-Pop-ups
wm-layer-lys-shell-osd = Lautstärke- und Helligkeitsanzeige
wm-layer-lys-shell-power-menu = Ein/Aus-Menü
wm-layer-lys-shell-portal-dialog = Portal-Dialoge
wm-layer-lys-shell-file-chooser = Dateiauswahl
wm-layer-lys-shell-print = Druckdialog
wm-layer-lys-shell-share-picker = Auswahl für Bildschirmfreigabe
wm-layer-lys-shell-screenshot-reading = Texterkennung
wm-layer-lys-shell-screenshot-thumbnail = Bildschirmfoto-Vorschau
wm-layer-lys-shell-color-picker = Farbwähler
wm-layer-lys-shell-region-overlay = Bereichsauswahl
wm-layer-lys-shell-window-pick = Fensterauswahl
wm-section-start = Start
    .description = Was mit der Sitzung startet, und ihre Umgebung
wm-section-xwayland = Xwayland
    .description = X11-Anwendungen
wm-section-palette = Palette
    .description = Farbnamen, die andere Schlüssel verwenden können

enum-wm-toggle-on = An
enum-wm-toggle-off = Aus
wm-status-not-running = Gespeichert. Hier läuft kein lys-wm; der nächste liest es ein.
wm-status-unreadable = Die Konfiguration von lys-wm lässt sich nicht lesen
wm-status-read-only = Die Konfiguration von lys-wm lässt sich hier nicht ändern, daher werden Änderungen nicht gespeichert
wm-status-not-applied = lys-wm hat die Änderung nicht übernommen
wm-column-widths-unreadable = Die Spaltenbreiten sind keine, die lys-wm annimmt; bearbeite sie als Text
wm-pointers-connected = Verbunden: { $devices }
wm-pointers-device = { $name } ({ $kind })
wm-pointers-class-pointer = Maus
wm-pointers-class-touchpad = Touchpad
wm-pointers-class-trackpoint = Trackpoint
wm-pointers-none = Weder Maus noch Touchpad noch Trackpoint ist verbunden.
wm-pointers-not-running = Hier läuft kein lys-wm, der sagen könnte, welche Geräte verbunden sind, daher werden alle Abschnitte angezeigt.
wm-pointers-outdated = Der laufende lys-wm ist älter und sagt nicht, welche Geräte verbunden sind, daher werden bis zur nächsten Anmeldung alle Abschnitte angezeigt.
wm-pointers-unreadable = Welche Geräte verbunden sind, ließ sich nicht lesen, daher werden alle Abschnitte angezeigt
wm-section-devices = Einzelne Geräte
    .description = Einstellungen, die ein Gerät für sich behält
wm-input-devices = Jedes Zeigegerät
    .description = Die eigenen Einstellungen eines Geräts gehen denen seiner Art oben vor; was es offen lässt, folgt ihnen.
wm-input-devices-keyboard = Jede Tastatur
    .description = Die eigene Belegung einer Tastatur geht der oben vor; was sie offen lässt, folgt ihr.
wm-device-absent = Nicht verbunden
    .description = Dieses Gerät hat eigene Einstellungen; sie gelten wieder, sobald es verbunden ist.
wm-device-forget = Die Einstellungen dieses Geräts vergessen
wm-devices-none = Kein Gerät ist verbunden oder hat eigene Einstellungen.
wm-devices-not-running = Hier läuft kein lys-wm, der sagen könnte, welche Geräte verbunden sind; die Geräte mit eigenen Einstellungen werden aufgeführt.
wm-devices-outdated = Der laufende lys-wm ist älter und sagt nicht, welche Geräte verbunden sind; bis zur nächsten Anmeldung werden die Geräte mit eigenen Einstellungen aufgeführt.
wm-devices-unreadable = Welche Geräte verbunden sind, ließ sich nicht lesen, daher werden nur die Geräte mit eigenen Einstellungen aufgeführt
wm-devices-reload = Erneut nach Geräten suchen
wm-rules-pick = Fenster auswählen …
wm-rules-picking = Klicke auf das Fenster, für das eine Regel entstehen soll. Escape bricht ab.
wm-rules-pick-none = Der Klick traf kein Fenster.
wm-rules-pick-gone = Das Fenster wurde geschlossen, bevor es gelesen werden konnte.
wm-rules-pick-unnamed = Dieses Fenster hat weder eine App-ID noch einen Titel, auf den eine Regel passen könnte.
wm-rules-pick-not-running = Hier läuft kein lys-wm, in dem ein Fenster ausgewählt werden könnte.
wm-rules-pick-failed = Das Fenster konnte nicht ausgewählt werden
wm-rules-applies-to = Gilt für
wm-rules-then = Dann
wm-rules-more-conditions = Weitere Bedingungen
wm-rules-fewer-conditions = Weniger Bedingungen
wm-rules-group-placement = Platzierung
wm-rules-group-appearance = Aussehen
wm-rules-group-behaviour = Verhalten
wm-rules-group-performance = Leistung
wm-rules-add-effect = Wirkung hinzufügen …
wm-rules-no-effects = Diese Regel ändert noch nichts: Füge eine Wirkung hinzu.
wm-rules-remove-effect = Diese Wirkung entfernen
wm-rules-enabled = Diese Regel anwenden
wm-rules-apps-running = Laufend
wm-rules-border = Rahmen
wm-rules-blur-look = Unschärfe-Aussehen
wm-preview-windows = Fenstervorschau
wm-preview-unreadable = Die Vorschau kann { $setting } mit dem Wert { $value } nicht zeichnen.
wm-source-file = 
    .description = In der Konfigurationsdatei von lys-wm gesetzt

wm-gaps-inner = Innerer Abstand
    .description = Pixel zwischen gekachelten Fenstern
    .keywords = Abstand, Innenabstand, Rand, Lücke
wm-gaps-outer = Äußerer Abstand
    .description = Eine Zahl oder [oben, rechts, unten, links]
wm-borders-width = Rahmenbreite
    .description = In Pixeln; 0 zeichnet keinen
wm-borders-active = Rahmen im Fokus
    .description = Eine Farbe oder Farben und ein Winkel für einen Verlauf
wm-borders-inactive = Rahmen ohne Fokus
    .description = Eine Farbe oder Farben und ein Winkel für einen Verlauf
wm-decoration-rounding = Eckenradius
    .description = In Pixeln
wm-decoration-active-opacity = Deckkraft im Fokus
    .description = Von 0 bis 1
wm-decoration-inactive-opacity = Deckkraft ohne Fokus
    .description = Von 0 bis 1
wm-decoration-fullscreen-opacity = Deckkraft im Vollbild
    .description = Von 0 bis 1
wm-decoration-dim-inactive = Fenster ohne Fokus abdunkeln
    .description = Jedes Fenster ohne Fokus abdunkeln
wm-decoration-dim-strength = Abdunklungsstärke
    .description = Von 0 bis 1
wm-decoration-shadow-enabled = Schatten
    .description = Einen Schatten unter Fenstern zeichnen
wm-decoration-shadow-range = Schattenreichweite
    .description = In Pixeln
wm-decoration-shadow-render-power = Schattenabfall
    .description = Von 1 bis 4; höher verblasst schneller
wm-decoration-shadow-sharp = Scharfe Schatten
    .description = Ohne weichen Rand
wm-decoration-shadow-scale = Schattenskalierung
    .description = Über 0, bis 1
wm-decoration-shadow-offset = Schattenversatz
    .description = Wie weit der Schatten rechts und unter das Fenster fällt, in Pixeln; negativ verschiebt ihn nach links oder oben
wm-shadow-offset-x = Nach rechts
wm-shadow-offset-y = Nach unten
wm-decoration-shadow-color = Schattenfarbe
    .description = #rrggbb[aa] oder ein Palettenname; setzt auch die Farbe ohne Fokus, sofern diese nicht gesetzt ist
wm-decoration-shadow-color-inactive = Schattenfarbe ohne Fokus
    .description = #rrggbb[aa] oder ein Palettenname
wm-decoration-blur-enabled = Unschärfe
    .description = Hinter jedem durchscheinenden Fenster weichzeichnen, nicht nur hinter denen, die es anfordern
    .keywords = Milchglas, Transparenz, durchscheinend
wm-decoration-blur-size = Unschärfegröße
    .description = In Pixeln
wm-decoration-blur-passes = Unschärfedurchgänge
    .description = Von 1 bis 4
wm-decoration-blur-xray = Röntgen-Unschärfe
    .description = Nur das Hintergrundbild weichzeichnen, nicht die Fenster darunter
wm-decoration-blur-tint = Unschärfe-Tönung
    .description = #rrggbb[aa] oder ein Palettenname
wm-decoration-blur-saturation = Unschärfe-Sättigung
    .description = Von 0 bis 4; 1 lässt sie unverändert
wm-decoration-blur-brightness = Unschärfe-Helligkeit
    .description = Von 0 bis 4; 1 lässt sie unverändert
wm-decoration-blur-contrast = Unschärfe-Kontrast
    .description = Von 0 bis 4; 1 lässt ihn unverändert
wm-decoration-blur-grain = Unschärfe-Körnung
    .description = Von 0 bis 1
wm-animations-enabled = Animationen
    .description = Fenster, Arbeitsflächen und Ebenen animieren
wm-animations-reduced-motion = Reduzierte Bewegung
    .description = Überblenden statt bewegen
    .keywords = Barrierefreiheit, Bewegungskrankheit
wm-overview-blur = Übersicht weichzeichnen
    .description = Weichzeichnen, was hinter der Übersicht liegt
wm-overview-dim = Abdunklung der Übersicht
    .description = Von 0 bis 1
wm-drag-preview-enabled = Ziehvorschau
    .description = Anzeigen, wo ein gezogenes Fenster landen wird
wm-drag-preview-opacity = Deckkraft der Vorschau
    .description = Von 0 bis 1
wm-drag-preview-blur = Vorschau weichzeichnen
    .description = Hinter der Vorschau weichzeichnen
wm-drag-preview-snap-distance = Einrastabstand
    .description = In Pixeln von einem Rand
wm-autotile-enabled = Automatisches Kacheln
    .description = Ein Fenster entlang seiner längeren Seite teilen
wm-autotile-ephemeral = Flüchtige Gruppen
    .description = Eine Gruppe mit nur einem Fenster löst sich auf
wm-autotile-min-trigger = Mindestgröße zum Teilen
    .description = In Pixeln
wm-autotile-divisor = Teilungsdivisor
    .description = Fenster, die schmaler als dieser Anteil der Ausgabe sind, werden nicht weiter geteilt
wm-tabs-height = Höhe der Reiterleiste
    .description = In Pixeln
wm-tabs-padding = Innenabstand der Reiterleiste
    .description = In Pixeln
wm-tabs-text-height = Texthöhe der Reiter
    .description = In Pixeln
wm-tabs-text-padding = Textabstand der Reiter
    .description = In Pixeln
wm-tabs-font = Reiterschrift
    .description = Pfad zu einer Schriftdatei; leer verwendet den Standard
wm-tabs-icon-theme = Symbolthema der Reiter
    .description = Leer verwendet den Standard
wm-input-keyboard-layout = Tastaturbelegungen
    .description = Die Belegungen, mit denen du schreibst, in der Reihenfolge, in der umgeschaltet wird; füge eine über ihren Namen hinzu, Varianten wie Dänisch (ohne Tottasten) eingeschlossen
    .keywords = Tastenbelegung, Sprache, Eingabequelle
wm-input-sources-misaligned = Diese Belegungen und Varianten passen nicht zusammen, es gibt mehr Varianten als Belegungen, daher stehen beide wie geschrieben da; wähle eine Belegung oder Variante, um sie neu zu setzen
wm-input-keyboard-variant = Tastaturvariante
    .description = XKB-Variante
wm-input-keyboard-options = Tastaturoptionen
    .description = XKB-Optionen, etwa caps:escape
wm-input-keyboard-model = Tastaturmodell
    .description = XKB-Modell
wm-input-keyboard-rules = Tastaturregeln
    .description = XKB-Regeln
wm-input-keyboard-repeat-rate = Wiederholrate
    .description = Tasten pro Sekunde
    .keywords = Tastenwiederholung, Typematic
wm-input-keyboard-repeat-delay = Wiederholverzögerung
    .description = Millisekunden, bevor eine gehaltene Taste wiederholt wird
wm-input-pointer-natural-scroll = Natürliches Scrollen
    .description = Der Inhalt folgt den Fingern
    .keywords = umgekehrtes Scrollen, invertiertes Scrollen, Mausrad
wm-input-pointer-middle-emulation = Mittelklick-Emulation
    .description = Links und rechts gleichzeitig klicken die mittlere Taste
wm-input-pointer-accel-profile = Beschleunigungsprofil
    .description = Flach bewegt den Zeiger so weit wie das Gerät; adaptiv beschleunigt ihn bei schnellerer Bewegung
wm-input-pointer-accel-speed = Beschleunigung
    .description = Von -1 bis 1
wm-input-pointer-accel-points = Eigene Beschleunigungskurve
    .description = Die Zeigergeschwindigkeit bei jeder Stufe der Gerätegeschwindigkeit, anstelle des Profils
wm-input-pointer-scroll-factor = Scrollgeschwindigkeit
    .description = Über 0, bis 100; 1 lässt sie unverändert
wm-input-trackpoint-natural-scroll = Natürliches Scrollen
    .description = Der Inhalt folgt den Fingern
    .keywords = umgekehrtes Scrollen, invertiertes Scrollen, Mausrad
wm-input-trackpoint-middle-emulation = Mittelklick-Emulation
    .description = Links und rechts gleichzeitig klicken die mittlere Taste
wm-input-trackpoint-accel-profile = Beschleunigungsprofil
    .description = Flach bewegt den Zeiger so weit wie das Gerät; adaptiv beschleunigt ihn bei schnellerer Bewegung
wm-input-trackpoint-accel-speed = Beschleunigung
    .description = Von -1 bis 1
wm-input-trackpoint-accel-points = Eigene Beschleunigungskurve
    .description = Die Zeigergeschwindigkeit bei jeder Stufe der Gerätegeschwindigkeit, anstelle des Profils
wm-input-trackpoint-scroll-factor = Scrollgeschwindigkeit
    .description = Über 0, bis 100; 1 lässt sie unverändert
wm-input-trackpoint-scroll-method = Scrollmethode
    .description = Wie der Trackpoint scrollt: mit einer gehaltenen Taste oder gar nicht
wm-input-touchpad-natural-scroll = Natürliches Scrollen
    .description = Der Inhalt folgt den Fingern
    .keywords = umgekehrtes Scrollen, invertiertes Scrollen, Trackpad
wm-input-touchpad-middle-emulation = Mittelklick-Emulation
    .description = Links und rechts gleichzeitig klicken die mittlere Taste
wm-input-touchpad-accel-profile = Beschleunigungsprofil
    .description = Flach bewegt den Zeiger so weit wie der Finger; adaptiv beschleunigt ihn bei schnellerer Bewegung
wm-input-touchpad-accel-speed = Beschleunigung
    .description = Von -1 bis 1
wm-input-touchpad-accel-points = Eigene Beschleunigungskurve
    .description = Die Zeigergeschwindigkeit bei jeder Stufe der Gerätegeschwindigkeit, anstelle des Profils
wm-input-touchpad-scroll-factor = Scrollgeschwindigkeit
    .description = Über 0, bis 100; 1 lässt sie unverändert
wm-input-touchpad-tap = Tippen zum Klicken
    .description = Ein Tippen klickt, ohne das Touchpad herunterzudrücken
    .keywords = Tippen zum Klicken, Trackpad
wm-input-touchpad-drag-lock = Ziehsperre
    .description = Ein Tippen-Ziehen bleibt beim Anheben des Fingers bestehen
wm-input-touchpad-disable-while-typing = Beim Tippen deaktivieren
    .description = Das Touchpad nach jedem Tastendruck kurz ignorieren
wm-input-touchpad-click-method = Klickmethode
    .description = Wie ein Druck die Taste wählt: nach der Stelle oder nach der Anzahl der Finger
wm-input-touchpad-scroll-method = Scrollmethode
    .description = Wie das Touchpad scrollt: zwei Finger, der Rand oder eine gehaltene Taste
wm-binds = Tastenbelegungen
    .description = Eine Liste aus Kombination, Aktion und Flags; eine Konfiguration ohne welche hat keine
wm-submaps = Untermodi
    .description = Benannte Sätze von Tastenbelegungen
wm-gestures = Gesten
    .description = Wischen, Zusammenziehen und Halten auf dem Touchpad
wm-outputs = Ausgaberegeln
    .description = Jede Ausgabe übernimmt die erste Regel, die auf sie passt
wm-color-management = Farbverwaltung
    .description = Clients das Farbverwaltungsprotokoll anbieten
wm-tearing = Tearing
    .description = Fenster, die es anfordern, im Vollbild Tearing erlauben
wm-gpu = GPU
    .description = Eine zu verwendende Karte oder ein Render-Knoten; leer wählt automatisch
wm-lid-action = Aktion beim Zuklappen
    .description = Was im Akkubetrieb ohne externen Bildschirm beim Zuklappen geschieht: Bereitschaft, Ruhezustand, Hybrid-Schlaf, Bereitschaft dann Ruhezustand, oder nichts
wm-rules = Regeln
    .description = Eine Liste aus Bedingung und Wirkung
wm-layer-rules = Regeln
    .description = Jede Regel ändert, wie die Flächen der Shell aussehen, für die sie gilt
wm-autostart = Autostart
    .description = Befehle, die ausgeführt werden, sobald die Sitzung läuft
wm-env = Umgebung
    .description = Variablen für lys-wm und die Apps, die es startet; eine Änderung erreicht danach gestartete Apps, nicht bereits laufende
wm-env-name-placeholder = Variable, etwa GDK_BACKEND
wm-env-name-invalid = Ein Variablenname besteht nur aus Buchstaben, Ziffern und _ und beginnt nicht mit einer Ziffer; bis dahin wird er nicht gespeichert
wm-env-moz-wayland = Firefox läuft unter Wayland
wm-env-qt-wayland = Qt-Apps laufen unter Wayland
wm-env-gdk-wayland = GTK-Apps laufen unter Wayland
wm-env-sdl-wayland = SDL-Spiele laufen unter Wayland
wm-env-electron-wayland = Electron-Apps laufen unter Wayland, wo sie es können
wm-env-java-tiling = Java-Apps zeichnen ihre Fenster richtig
wm-xwayland-enabled = Xwayland
    .description = X11-Anwendungen ausführen
wm-xwayland-zero-scaling = Nullskalierung
    .description = X11-Apps zeichnen in den echten Pixeln des Bildschirms, statt unscharf hochskaliert zu werden; auf einem skalierten Bildschirm wirken sie klein, wenn sie sich nicht selbst skalieren
wm-palette = Palette
    .description = Benannte Farben, die andere Schlüssel verwenden können

## Structured keys
wm-section-gestures = Gesten
    .description = Touchpad-Gesten und was sie steuern
wm-gestures-swipe = Wischen
    .description = Wischgesten auf dem Touchpad und was jede steuert oder ausführt
wm-gestures-pinch = Zusammenziehen
    .description = Zusammenziehgesten auf dem Touchpad und was jede steuert oder ausführt
wm-gestures-hold = Halten
    .description = Auf dem Touchpad gehaltene Finger und die Aktion, die jede ausführt
wm-gestures-distance = Gestenweg
    .description = Touchpad-Einheiten für ein ganzes Wischen, etwa 12 pro Millimeter
wm-card-number = Eintrag { $number }
wm-side-top = Oben
wm-side-right = Rechts
wm-side-bottom = Unten
wm-side-left = Links
wm-border-add-color = Farbe hinzufügen
wm-border-angle = Winkel
wm-binds-action = Aktion
wm-binds-chord = Kombination
wm-binds-flags = Flags
wm-binds-cooldown = Abklingzeit (ms)
wm-binds-description = Beschreibung
wm-binds-new = Neues Tastenkürzel
wm-binds-long-press = Langes Drücken (ms)
wm-gestures-action = Aktion
wm-gestures-direction = Richtung
wm-gestures-drive = Steuert
wm-gestures-fingers = Finger
wm-gestures-modifiers = Modifikatoren
wm-gestures-scope = Bereich der Übersicht
wm-outputs-bitdepth = Farbtiefe
    .description = 8 oder 10 Bit pro Kanal; 10 glättet Verläufe unter Nachtlicht
wm-outputs-disabled = Deaktiviert
wm-outputs-hdr = HDR
    .description = Den Bildschirm immer in HDR betreiben oder nur bei HDR-Inhalten im Vollbild
wm-outputs-hdr-peak = HDR-Spitzenhelligkeit
    .description = Mit HDR: die Spitzenhelligkeit des Bildschirms in cd/m², falls seine EDID sie falsch angibt
wm-outputs-icc = ICC-Profil
    .description = Ein Matrix-Shaper-ICC-Profil für einen SDR-Bildschirm
wm-outputs-hot-corners = Aktive Ecken
    .description = Aus schaltet alle aktiven Ecken auf diesem Bildschirm ab; An ändert einige davon hier
wm-outputs-hot-corners-top-left = Ecke oben links
wm-outputs-hot-corners-top-left-action = Aktion
wm-outputs-hot-corners-top-left-modifiers = Modifikatoren
wm-outputs-hot-corners-top-right = Ecke oben rechts
wm-outputs-hot-corners-top-right-action = Aktion
wm-outputs-hot-corners-top-right-modifiers = Modifikatoren
wm-outputs-hot-corners-bottom-left = Ecke unten links
wm-outputs-hot-corners-bottom-left-action = Aktion
wm-outputs-hot-corners-bottom-left-modifiers = Modifikatoren
wm-outputs-hot-corners-bottom-right = Ecke unten rechts
wm-outputs-hot-corners-bottom-right-action = Aktion
wm-outputs-hot-corners-bottom-right-modifiers = Modifikatoren
wm-outputs-match = Bedingung
wm-outputs-mirror = Spiegeln
    .description = Ein anderer Bildschirm, dessen Bild dieser statt seiner eigenen Arbeitsfläche zeigt
wm-outputs-mode = Modus
wm-outputs-position = Position
wm-outputs-reserved = Reservierter Bereich
    .description = Logische Pixel, die frei von gekachelten und maximierten Fenstern bleiben: oben, rechts, unten, links
wm-outputs-scale = Skalierung
wm-outputs-sdr-brightness = SDR-Helligkeit
    .description = Mit HDR: die cd/m², mit denen SDR-Weiß angezeigt wird
wm-outputs-shader = Shader
    .description = Ein Fragment-Shader für das endgültige Bild des Bildschirms
wm-outputs-transform = Drehung
    .description = Wie das Panel gedreht oder gespiegelt ist
wm-outputs-vrr = Variable Bildwiederholrate
    .description = Adaptive Synchronisation: aus, immer an oder nur für ein Spiel oder Video im Vollbild
wm-rules-animation = Animation
wm-rules-animation-in = Öffnungsanimation
wm-rules-animation-out = Schließanimation
wm-rules-app-id = App-ID
wm-rules-block-out-from = Vor Aufnahmen verbergen
wm-rules-blur = Unschärfe
wm-rules-blur-look-brightness = Unschärfe-Helligkeit
wm-rules-blur-look-contrast = Unschärfe-Kontrast
wm-rules-blur-look-grain = Unschärfe-Körnung
wm-rules-blur-look-refraction-amount = Brechungsstärke
wm-rules-blur-look-refraction-bezel = Brechungsrand
wm-rules-blur-look-saturation = Unschärfe-Sättigung
wm-rules-blur-look-specular-strength = Glanzstärke
wm-rules-blur-look-specular-width = Glanzbreite
wm-rules-blur-look-tint = Unschärfe-Tönung
wm-rules-border-active = Aktiver Rahmen
wm-rules-border-inactive = Inaktiver Rahmen
wm-rules-border-width = Rahmenbreite
wm-rules-dim-around = Umgebung abdunkeln
wm-rules-dim-inactive = Abdunkeln, wenn inaktiv
wm-rules-float = Schwebend
wm-rules-mica = Mica
wm-rules-opacity = Deckkraft
wm-rules-size = Größe
wm-rules-tag = Tag
wm-rules-tearing = Tearing
wm-rules-touch-gestures = Touch-Gesten
wm-rules-tile = Kacheln
wm-rules-vrr = Variable Bildwiederholrate
wm-rules-title = Titel
wm-rules-exclude-app-id = Außer App-ID
wm-rules-exclude-is-captured = Außer aufgenommen
wm-rules-exclude-is-floating = Außer schwebend
wm-rules-exclude-is-focused = Außer fokussiert
wm-rules-exclude-is-fullscreen = Außer Vollbild
wm-rules-exclude-is-urgent = Außer dringend
wm-rules-exclude-initial-app-id = Außer anfänglicher App-ID
wm-rules-exclude-initial-title = Außer anfänglichem Titel
wm-rules-exclude-tag = Außer Tag
wm-rules-exclude-title = Außer Titel
wm-rules-exclude-xwayland = Außer Xwayland
wm-rules-focus = Fokus
wm-rules-fullscreen = Vollbild
wm-rules-group = Gruppe
wm-rules-idle-inhibit = Leerlauf verhindern
wm-rules-initial-app-id = Anfängliche App-ID
wm-rules-initial-title = Anfänglicher Titel
wm-rules-is-captured = Wird aufgenommen
wm-rules-is-floating = Ist schwebend
wm-rules-is-focused = Ist fokussiert
wm-rules-is-fullscreen = Ist im Vollbild
wm-rules-is-urgent = Ist dringend
wm-rules-keep-aspect = Seitenverhältnis beibehalten
wm-rules-max-size = Maximale Größe
wm-rules-min-size = Minimale Größe
wm-rules-maximized = Maximiert
wm-rules-output = Ausgabe
wm-rules-pin = Anheften
wm-rules-position = Position
wm-rules-rounding = Eckenradius
wm-rules-scroll-factor = Scrollfaktor
wm-rules-shadow = Schatten
wm-rules-suppress = Ignorierte Anfragen
wm-rules-workspace = Arbeitsfläche
wm-rules-xwayland = Xwayland
wm-section-focus = Fokus
    .description = Was ein Fenster bekommt, das den Fokus anfordert
wm-focus-on-activate = Bei Aktivierung
    .description = Ein anfragendes Fenster fokussieren, wenn Sie die anfragende App gerade benutzt haben, sonst als dringend markieren; oder immer als dringend markieren
wm-rules-on-activate = Bei Aktivierung
    .description = Was dieses Fenster bekommt, wenn es den Fokus anfordert: Fokus, dringend oder gar nichts
wm-section-app-shortcuts = App-Tastenkürzel
    .description = Tastenkürzel, die Apps über das Portal registriert haben, jeweils über eine globale Belegung ausgelöst
wm-app-shortcuts = Registrierte Tastenkürzel
    .description = Eine Kombination aufnehmen, um sie zu belegen, Rücktaste, um die Belegung zu entfernen
wm-app-shortcuts-none = Noch keine App hat ein Tastenkürzel registriert
wm-app-shortcuts-hotkey = Die App hält diese Taste selbst, eingestellt in ihrem Portal-Dialog
wm-rules-content = Inhalt
    .description = Was das Fenster zeigt (Foto, Video oder Spiel), vorrangig vor seiner eigenen Angabe
wm-app-shortcuts-reload = Erneut suchen
wm-section-window-behavior = Fensterverhalten
    .description = Wie sich neue, schwebende und Vollbildfenster verhalten
wm-section-titlebars = Titelleisten
    .description = Leisten mit Knöpfen über schwebenden Fenstern
wm-section-mica = Mica
    .description = Starke Unschärfe des Hintergrundbilds hinter durchscheinenden Fenstern
wm-section-hot-corners = Aktive Ecken
    .description = Was passiert, wenn der Zeiger in eine Bildschirmecke geschoben wird
wm-section-layout = Anordnung
    .description = Wie ein neuer Arbeitsbereich seine Fenster anordnet
wm-section-cursor = Zeiger
    .description = Wann der Zeiger verschwindet und wie man ihn findet
wm-focus-follows-mouse = Fokus folgt dem Zeiger
    .description = Ein Fenster unter dem Zeiger erhält den Fokus; locker wartet, bis der Zeiger ein anderes Fenster betritt
wm-focus-raise-on-hover = Beim Überfahren anheben
    .description = Der Fokus beim Überfahren holt auch ein schwebendes Fenster nach vorn
wm-focus-warp-pointer = Zeiger mit Fokus bewegen
    .description = Den Zeiger auf ein per Tastenkürzel fokussiertes Fenster setzen, oder nur beim Arbeitsbereichswechsel
wm-focus-on-close = Fokus nach dem Schließen
    .description = Welches Fenster den Fokus erhält, wenn das fokussierte schließt
wm-fullscreen-new-window = Neues Fenster über Vollbild
    .description = Ob ein Fenster über einem Vollbild- oder maximierten Fenster dahinter bleibt, übernimmt oder es beendet
wm-hold-opening = Auf Nachbarn warten
    .description = Ein neues Fenster wartet bis zu 100 ms, bis verkleinerte Nachbarn neu gezeichnet sind
wm-floating-step = Schrittweite schwebender Fenster
    .description = Pixel, um die ein Tastenkürzel ein schwebendes Fenster verschiebt
wm-section-swallow = Terminal-Verschlucken
    .description = Ein Terminal verbirgt sich hinter dem Fenster, das es öffnet, bis dieses sich schließt
wm-swallow-terminals = Verschluckende Terminals
    .description = Ein aus einem dieser Terminals gestartetes Fenster nimmt dessen Platz ein, bis es sich schließt
wm-swallow-except-title = Titel, die nie verschlucken
    .description = Ein regulärer Ausdruck auf den Titel des neuen Fensters; passende Fenster öffnen sich neben dem Terminal
wm-swallow-terminals-none = Kein Terminal verschluckt die Fenster, die es startet
wm-swallow-terminals-add = Terminal hinzufügen
wm-swallow-terminals-written = Ein von Hand geschriebenes Muster, das keine Liste von Terminals ausdrücken kann
wm-swallow-terminals-replace = Durch eine Liste ersetzen
wm-titlebars-floating = Titelleisten zeigen
    .description = Leisten mit Knöpfen über schwebenden Fenstern
wm-titlebars-height = Titelleistenhöhe
    .description = Pixel, vom schwebenden Fenster abgezogen
wm-titlebars-buttons = Knöpfe
    .description = Welche Knöpfe die Leiste zeigt, in Reihenfolge
wm-titlebars-side = Knopfseite
    .description = An welchem Ende der Leiste die Knöpfe sitzen
wm-titlebars-double-click = Doppelklick
    .description = Was ein Doppelklick auf die Leiste bewirkt
wm-titlebars-active-background = Titelleiste-Hintergrund, fokussiert
    .description = Farbe: Titelleiste, fokussiert
wm-titlebars-active-text = Titelleiste-Text, fokussiert
    .description = Textfarbe: Titelleiste, fokussiert
wm-titlebars-inactive-background = Titelleiste-Hintergrund, unfokussiert
    .description = Farbe: Titelleiste, unfokussiert
wm-titlebars-inactive-text = Titelleiste-Text, unfokussiert
    .description = Textfarbe: Titelleiste, unfokussiert
wm-titlebars-urgent-background = Titelleiste-Hintergrund, dringend
    .description = Farbe: Titelleiste, dringend
wm-titlebars-urgent-text = Titelleiste-Text, dringend
    .description = Textfarbe: Titelleiste, dringend
wm-tabs-active-background = Tab-Hintergrund, fokussiert
    .description = Farbe: Tab, fokussiert
wm-tabs-active-text = Tab-Text, fokussiert
    .description = Textfarbe: Tab, fokussiert
wm-tabs-inactive-background = Tab-Hintergrund, unfokussiert
    .description = Farbe: Tab, unfokussiert
wm-tabs-inactive-text = Tab-Text, unfokussiert
    .description = Textfarbe: Tab, unfokussiert
wm-tabs-urgent-background = Tab-Hintergrund, dringend
    .description = Farbe: Tab, dringend
wm-tabs-urgent-text = Tab-Text, dringend
    .description = Textfarbe: Tab, dringend
wm-titlebars-lights-close = Farbe: Schließen
    .description = Farbe des Punkts für Schließen
wm-titlebars-lights-minimize = Farbe: Minimieren
    .description = Farbe des Punkts für Minimieren
wm-titlebars-lights-maximize = Farbe: Maximieren
    .description = Farbe des Punkts für Maximieren
wm-decoration-blur-live-tiled = Live-Unschärfe für gekachelte Fenster
    .description = Hinter jedem gekachelten Fenster weichzeichnen statt der gemeinsamen Hintergrundunschärfe
wm-decoration-blur-mica-enabled = Mica
    .description = Durchscheinende Fenster über einer starken Unschärfe des Hintergrunds zeichnen
wm-decoration-blur-mica-size = Mica-Unschärfegröße
    .description = Abstand zwischen den Unschärfeproben in Pixel
wm-decoration-blur-mica-passes = Mica-Unschärfedurchgänge
    .description = Jeder Durchgang verdoppelt die Reichweite
wm-decoration-blur-mica-tint = Mica-Tönung
    .description = Farbe über dem Mica-Hintergrund
wm-drag-preview-snap-windows = An Fenstern einrasten
    .description = Schwebende Fenster rasten auch neben oder bündig mit anderen ein
wm-drag-preview-drag-switch-width = Randwechselzone
    .description = Pixel am Bildschirmrand ohne Nachbarn, in denen Halten beim Ziehen den Arbeitsbereich wechselt
wm-drag-preview-drag-switch-frames = Randwechselverzögerung
    .description = Bilder, die ein Ziehen am Rand gehalten werden muss
wm-hot-corners-corners = Ecken
    .description = Eine Ecke wählen, dann, was das Hineinschieben des Zeigers bewirkt
wm-hot-corners-top-left = Oben links
    .description = Aktion, wenn der Zeiger in diese Ecke geschoben wird
wm-hot-corners-top-right = Oben rechts
    .description = Aktion, wenn der Zeiger in diese Ecke geschoben wird
wm-hot-corners-bottom-left = Unten links
    .description = Aktion, wenn der Zeiger in diese Ecke geschoben wird
wm-hot-corners-bottom-right = Unten rechts
    .description = Aktion, wenn der Zeiger in diese Ecke geschoben wird
wm-hot-corners-corner-action = Aktion
    .description = Was die Ecke tut
wm-hot-corners-corner-modifiers = Gehaltene Tasten
    .description = Tasten, die gehalten werden müssen
wm-hot-corners-pressure = Eckendruck
    .description = Wie weit der Zeiger in eine Ecke drücken muss
wm-hot-corners-fullscreen = Ecken im Vollbild
    .description = Ecken lösen auch bei fokussiertem Vollbildfenster aus
wm-showdesktop-peek = Schreibtisch-Rand
    .description = Prozent jedes Fensters, das beim Zeigen des Schreibtischs sichtbar bleibt
wm-switcher-open-delay = Verzögerung des Fensterwechslers
    .description = Millisekunden, die die Tasten gehalten werden müssen
wm-layout = Anordnung neuer Arbeitsbereiche
    .description = Teilungen und Tabgruppen oder ein scrollender Spaltenstreifen
wm-scroll-center = Fokussierte Spalte zentrieren
    .description = Wann der Streifen die fokussierte Spalte in die Mitte rollt
wm-input-keyboard-numlock = Num-Taste an
    .description = Num Lock beim Start und nach jedem Neuladen einschalten
wm-input-keyboard-sticky-keys = Einrastende Tasten
    .description = Eine allein getippte Sondertaste gilt für die nächste Taste; zweimal getippt rastet sie ein
wm-input-keyboard-slow-keys = Langsame Tasten
    .description = Millisekunden, die eine Taste gehalten werden muss; 0 ist aus
wm-input-pointer-tap = Tippen zum Klicken
    .description = Tippen klickt, ohne herunterzudrücken
wm-input-pointer-drag-lock = Ziehsperre
    .description = Tippen und Ziehen läuft weiter, nachdem der Finger abhebt
wm-input-pointer-disable-while-typing = Beim Tippen aus
    .description = Das Gerät beim Tippen ignorieren
wm-input-pointer-disabled-on-external-mouse = Aus bei angeschlossener Maus
    .description = Dieses Gerät ausschalten, solange ein anderer Zeiger angeschlossen ist
wm-input-pointer-left-handed = Linkshändig
    .description = Primäre und sekundäre Taste tauschen
wm-input-pointer-click-method = Klickmethode
    .description = Wie ein Klick erkannt wird: Tastenbereiche oder Fingeranzahl
wm-input-pointer-scroll-method = Bildlaufmethode
    .description = Zwei Finger, der Rand oder eine gehaltene Taste
wm-input-pointer-scroll-button-lock = Bildlauftaste schaltet um
    .description = Ein Klick auf die Bildlauftaste schaltet den Bildlauf ein und aus
wm-input-pointer-tap-button-map = Tipp-Tasten
    .description = Welche Tasten Tippen mit ein, zwei und drei Fingern drückt
wm-input-touchpad-disabled-on-external-mouse = Aus bei angeschlossener Maus
    .description = Dieses Gerät ausschalten, solange ein anderer Zeiger angeschlossen ist
wm-input-touchpad-left-handed = Linkshändig
    .description = Primäre und sekundäre Taste tauschen
wm-input-touchpad-scroll-button-lock = Bildlauftaste schaltet um
    .description = Ein Klick auf die Bildlauftaste schaltet den Bildlauf ein und aus
wm-input-touchpad-tap-button-map = Tipp-Tasten
    .description = Welche Tasten Tippen mit ein, zwei und drei Fingern drückt
wm-input-trackpoint-tap = Tippen zum Klicken
    .description = Tippen klickt, ohne herunterzudrücken
wm-input-trackpoint-drag-lock = Ziehsperre
    .description = Tippen und Ziehen läuft weiter, nachdem der Finger abhebt
wm-input-trackpoint-disable-while-typing = Beim Tippen aus
    .description = Das Gerät beim Tippen ignorieren
wm-input-trackpoint-disabled-on-external-mouse = Aus bei angeschlossener Maus
    .description = Dieses Gerät ausschalten, solange ein anderer Zeiger angeschlossen ist
wm-input-trackpoint-left-handed = Linkshändig
    .description = Primäre und sekundäre Taste tauschen
wm-input-trackpoint-click-method = Klickmethode
    .description = Wie ein Klick erkannt wird: Tastenbereiche oder Fingeranzahl
wm-input-trackpoint-scroll-button-lock = Bildlauftaste schaltet um
    .description = Ein Klick auf die Bildlauftaste schaltet den Bildlauf ein und aus
wm-input-trackpoint-tap-button-map = Tipp-Tasten
    .description = Welche Tasten Tippen mit ein, zwei und drei Fingern drückt
wm-gestures-edge-width = Berührungsrandzone
    .description = Pixel vom Bildschirmrand, in denen ein Randwischen beginnt
wm-cursor-hide-while-typing = Beim Tippen ausblenden
    .description = Den Zeiger beim Tippen in ein Fenster ausblenden
wm-cursor-hide-after = Bei Untätigkeit ausblenden
    .description = Millisekunden ohne Zeigerbewegung bis zum Ausblenden; 0 nie
wm-cursor-shake-to-find = Schütteln zum Finden
    .description = Schütteln der Maus vergrößert den Zeiger kurz
wm-hidden-frame-rate = Bildrate verdeckter Fenster
    .description = Bilder pro Sekunde für ganz verdeckte oder unsichtbare Fenster
wm-power-saver-frame-rate = Bildrate im Energiesparmodus
    .description = Höchste Bildrate im Energiesparprofil; 0 ist aus
wm-client-hotkeys = Globale App-Kürzel
    .description = Apps globale Kürzel erlauben, die das fokussierte Fenster nicht sieht
wm-side-all = Alle Seiten
wm-side-same = Auf allen Seiten gleich
wm-side-window = Fenster
enum-wm-accel-profile-adaptive = Adaptiv
enum-wm-accel-profile-flat = Gleichmäßig
enum-wm-buttons-close = Schließen
enum-wm-buttons-maximize = Maximieren
enum-wm-buttons-minimize = Minimieren
enum-wm-center-always = Immer
enum-wm-center-never = Nie
enum-wm-center-on-overflow = Bei Überlauf
enum-wm-click-method-button_areas = Tastenbereiche
enum-wm-click-method-clickfinger = Anzahl der Finger
enum-wm-click-method-none = Keine
enum-wm-double-click-float = Schwebend machen
enum-wm-double-click-maximize = Maximieren
enum-wm-follows-mouse-always = Immer unter dem Zeiger
enum-wm-follows-mouse-loose = Wenn der Zeiger ein anderes Fenster betritt
enum-wm-follows-mouse-off = Zum Fokussieren klicken
enum-wm-layout-scroll = Scrollende Spalten
enum-wm-layout-tree = Kachelbaum
enum-wm-lid-action-auto = Bereitschaft, dann Ruhezustand wenn möglich
enum-wm-lid-action-hibernate = Ruhezustand
enum-wm-lid-action-hybrid-sleep = Hybrider Ruhezustand
enum-wm-lid-action-ignore = Nichts tun
enum-wm-lid-action-suspend = Bereitschaft
enum-wm-lid-action-suspend-then-hibernate = Bereitschaft, dann Ruhezustand
enum-wm-new-window-behind = Öffnet sich dahinter
enum-wm-new-window-exit = Beendet den Vollbildmodus
enum-wm-new-window-take_over = Übernimmt den Vollbildmodus
enum-wm-on-activate-focus = Fokussieren
enum-wm-on-activate-urgent = Als dringend markieren
enum-wm-on-close-next = Das nächste Fenster
enum-wm-on-close-under_pointer = Das Fenster unter dem Zeiger
enum-wm-scroll-method-edge = Rand
enum-wm-scroll-method-none = Keine
enum-wm-scroll-method-on_button_down = Bei gedrückter Taste
enum-wm-scroll-method-two_finger = Zwei Finger
enum-wm-side-left = Links
enum-wm-side-right = Rechts
enum-wm-tap-button-map-lmr = Links, Mitte, rechts
enum-wm-tap-button-map-lrm = Links, rechts, Mitte
enum-wm-warp-pointer-off = Nie
enum-wm-warp-pointer-on_keyboard_focus = Wenn die Tastatur den Fokus bewegt
enum-wm-warp-pointer-on_workspace_switch = Beim Wechsel der Arbeitsfläche
enum-wm-flags-locked = Auch bei Sperre
enum-wm-flags-repeating = Wiederholt beim Halten
enum-wm-flags-release = Beim Loslassen
enum-wm-flags-pass = Erreicht auch das Fenster
enum-wm-flags-inhibit_proof = Auch wenn Kürzel gesperrt sind
enum-wm-scope-workspace = Diese Arbeitsfläche
enum-wm-scope-output = Dieser Bildschirm
enum-wm-scope-app = Diese App
enum-wm-toggle-fullscreen = Nur im Vollbild
enum-wm-vrr-off = Aus
enum-wm-vrr-on = Immer
enum-wm-vrr-fullscreen = Nur im Vollbild
enum-wm-transform-normal = Normal
enum-wm-transform-90 = Um 90° gedreht
enum-wm-transform-180 = Um 180° gedreht
enum-wm-transform-270 = Um 270° gedreht
enum-wm-transform-flipped = Gespiegelt
enum-wm-transform-flipped-90 = Gespiegelt, um 90° gedreht
enum-wm-transform-flipped-180 = Gespiegelt, um 180° gedreht
enum-wm-transform-flipped-270 = Gespiegelt, um 270° gedreht
enum-wm-content-none = Keiner
enum-wm-content-game = Spiel
enum-wm-content-photo = Foto
enum-wm-content-video = Video
enum-wm-on-activate-ignore = Ignorieren
settings-nav-wm-keyboard = Tastatur
settings-page-wm-keyboard = Tastatur
settings-nav-wm-pointer = Maus & Touchpad
settings-page-wm-pointer = Maus & Touchpad
wm-section-typing = Tippen
    .description = Wie schnell gehaltene Tasten wiederholen, und Num-Lock beim Start
wm-section-keyboard-accessibility = Barrierefreiheit
    .description = Tasten, die einrasten oder warten, um eine Taste nach der anderen zu tippen
wm-section-keyboard-model = Tastaturmodell
    .description = Tastaturmodell und Regeln, für die die Belegungen gebaut sind
wm-palette-new-name = Name der neuen Farbe
wm-palette-add = Farbe hinzufügen
wm-palette-reset = Zurück zur Farbe von lys-wm
wm-palette-taken = Es gibt schon eine Farbe namens { $name }
wm-palette-add-failed = Die Farbe { $name } wurde nicht hinzugefügt
wm-palette-reset-failed = Die Farbe { $name } wurde nicht zurückgesetzt
## What a bind does, by lys-wm action: $first is the first argument, or default without any; $args all of them; $rest all after the first
wm-action-focus = { $first ->
        [left] Das Fenster links fokussieren
        [right] Das Fenster rechts fokussieren
        [up] Das Fenster darüber fokussieren
        [down] Das Fenster darunter fokussieren
       *[other] { $args } fokussieren
    }
wm-action-move = { $first ->
        [left] Fenster nach links verschieben
        [right] Fenster nach rechts verschieben
        [up] Fenster nach oben verschieben
        [down] Fenster nach unten verschieben
       *[other] Fenster verschieben: { $args }
    }
wm-action-workspace = { $first ->
        [previous] Zur vorherigen Arbeitsfläche wechseln
        [empty] Zu einer leeren Arbeitsfläche wechseln
       *[other] Zu Arbeitsfläche { $args } wechseln
    }
wm-action-movetoworkspace = { $first ->
        [previous] Fenster auf die vorherige Arbeitsfläche verschieben
        [empty] Fenster auf eine leere Arbeitsfläche verschieben
       *[other] Fenster auf Arbeitsfläche { $args } verschieben
    }
wm-action-focusoutput = { $first ->
        [left] Den Bildschirm links fokussieren
        [right] Den Bildschirm rechts fokussieren
        [up] Den Bildschirm darüber fokussieren
        [down] Den Bildschirm darunter fokussieren
       *[other] Bildschirm { $args } fokussieren
    }
wm-action-movetooutput = { $first ->
        [left] Fenster auf den Bildschirm links verschieben
        [right] Fenster auf den Bildschirm rechts verschieben
        [up] Fenster auf den Bildschirm darüber verschieben
        [down] Fenster auf den Bildschirm darunter verschieben
       *[other] Fenster auf Bildschirm { $args } verschieben
    }
wm-action-moveworkspacetooutput = { $first ->
        [left] Arbeitsfläche auf den Bildschirm links verschieben
        [right] Arbeitsfläche auf den Bildschirm rechts verschieben
        [up] Arbeitsfläche auf den Bildschirm darüber verschieben
        [down] Arbeitsfläche auf den Bildschirm darunter verschieben
       *[other] Arbeitsfläche auf Bildschirm { $args } verschieben
    }
wm-action-swapworkspaces = Arbeitsflächen der Bildschirme { $args } tauschen
wm-action-scratchpad = { $first ->
        [hide] Das Scratchpad ausblenden
       *[other] Scratchpad { $rest } umschalten
    }
wm-action-movetoscratchpad = Fenster ins Scratchpad verschieben { $args }
wm-action-kill = Fenster schließen
wm-action-focuswindow = Fenster { $args } fokussieren
wm-action-minimize = { $first ->
        [default] Fenster minimieren
       *[other] Fenster minimieren: { $args }
    }
wm-action-restore = { $first ->
        [default] Das zuletzt minimierte Fenster wiederherstellen
       *[other] Fenster { $args } wiederherstellen
    }
wm-action-overview = { $first ->
        [show] Die Übersicht anzeigen
        [hide] Die Übersicht ausblenden
       *[other] Die Übersicht umschalten
    }
wm-action-closewindow = Fenster { $args } schließen
wm-action-dpms = { $first ->
        [on] Bildschirme einschalten
        [off] Bildschirme ausschalten
       *[other] Bildschirme ein- oder ausschalten
    }
wm-action-reload = Die Konfiguration neu laden
wm-action-dismissconfignotice = Den Konfigurationshinweis schließen
wm-action-exit = Die Sitzung beenden
wm-action-nextlayout = Zur nächsten Tastaturbelegung wechseln
wm-action-stickykeys = { $first ->
        [on] Einrastfunktion einschalten
        [off] Einrastfunktion ausschalten
       *[other] Einrastfunktion { $args }
    }
wm-action-slowkeys = Anschlagverzögerung auf { $args } ms setzen
wm-action-global = App-Tastenkürzel { $args }
wm-action-exec = { $args } ausführen
wm-action-launch = { $app } starten
wm-action-makegroup = { $first ->
        [h] Fenster nebeneinander gruppieren
        [v] Fenster gestapelt gruppieren
        [tab] Fenster als Reiter gruppieren
       *[other] Fenster gruppieren: { $args }
    }
wm-action-setlayout = { $first ->
        [tree] Das Kachel-Layout verwenden
        [scroll] Das Scroll-Layout verwenden
       *[other] Layout { $args } festlegen
    }
wm-action-columnwidth = Spaltenbreite { $args } festlegen
wm-action-consume = { $first ->
        [left] Fenster in die Spalte links verschieben
        [right] Fenster in die Spalte rechts verschieben
       *[other] Fenster in Spalte { $args } verschieben
    }
wm-action-expel = Fenster in eine eigene Spalte verschieben
wm-action-center = Die fokussierte Spalte zentrieren
wm-action-columndisplay = { $first ->
        [stack] Die Fenster der Spalte stapeln
        [tab] Die Spalte als Reiter anzeigen
       *[other] Die Spalte anzeigen als { $args }
    }
wm-action-changegroup = { $first ->
        [h] Die Gruppe nebeneinander anordnen
        [v] Die Gruppe gestapelt anordnen
        [tab] Die Gruppe als Reiter anordnen
        [toggletab] Reiter in der Gruppe umschalten
        [opposite] Die Gruppe andersherum ausrichten
       *[other] Gruppe ändern: { $args }
    }
wm-action-focustab = { $first ->
        [prev] Den vorherigen Reiter fokussieren
        [next] Den nächsten Reiter fokussieren
       *[other] Reiter { $args } fokussieren
    }
wm-action-focusparent = Die umgebende Gruppe fokussieren
wm-action-focuschild = Zurück in die Gruppe fokussieren
wm-action-swap = { $first ->
        [left] Fenster mit dem linken tauschen
        [right] Fenster mit dem rechten tauschen
        [up] Fenster mit dem darüber tauschen
        [down] Fenster mit dem darunter tauschen
       *[other] Fenster tauschen: { $args }
    }
wm-action-moveout = Fenster aus seiner Gruppe herausnehmen
wm-action-equalize = { $first ->
        [all] Jede Teilung gleichmäßig machen
       *[other] Die Gruppe gleichmäßig machen
    }
wm-action-locktab = { $first ->
        [on] Die Reitergruppe sperren
        [off] Die Reitergruppe entsperren
       *[other] Die Reitergruppe sperren oder entsperren
    }
wm-action-submap = { $first ->
        [reset] Den Modus verlassen
       *[other] Modus { $args } betreten
    }
wm-action-togglefloating = Fenster schweben lassen oder kacheln
wm-action-place = { $first ->
        [left] Fenster auf die linke Hälfte setzen
        [right] Fenster auf die rechte Hälfte setzen
        [top] Fenster auf die obere Hälfte setzen
        [bottom] Fenster auf die untere Hälfte setzen
        [topleft] Fenster in das obere linke Viertel setzen
        [topright] Fenster in das obere rechte Viertel setzen
        [bottomleft] Fenster in das untere linke Viertel setzen
        [bottomright] Fenster in das untere rechte Viertel setzen
        [fill] Fenster den Bildschirm füllen lassen
        [center] Fenster zentrieren
        [restore] Fenster an seinen alten Platz zurücksetzen
       *[other] Fenster platzieren: { $args }
    }
wm-action-focusfloating = { $first ->
        [on] Die schwebenden Fenster fokussieren
        [off] Die gekachelten Fenster fokussieren
       *[other] Den Fokus zwischen schwebenden und gekachelten Fenstern wechseln
    }
wm-action-fullscreen = { $first ->
        [on] Fenster im Vollbild anzeigen
        [off] Vollbild verlassen
       *[other] Vollbild umschalten
    }
wm-action-setprop = Fenstereigenschaft { $args } festlegen
wm-action-showdesktop = { $first ->
        [on] Den Desktop anzeigen
        [off] Die Fenster zurückholen
       *[other] Den Desktop anzeigen oder ausblenden
    }
wm-action-swallow = { $first ->
        [on] Das Terminal im Fenster ausblenden
        [off] Das ausgeblendete Terminal anzeigen
       *[other] Das verschluckte Terminal anzeigen oder ausblenden
    }
wm-action-pin = { $first ->
        [on] Fenster anheften
        [off] Fenster lösen
       *[other] Fenster anheften oder lösen
    }
wm-action-maximize = { $first ->
        [on] Fenster maximieren
        [off] Fenster wiederherstellen
       *[other] Maximieren umschalten
    }
wm-action-resizeactive = Fenstergröße ändern um { $args }
wm-action-movewindow = Fenster ziehen
wm-action-resizewindow = Fenstergröße durch Ziehen ändern
wm-action-zoom = { $first ->
        [in] Vergrößern
        [out] Verkleinern
        [reset] Zoom zurücksetzen
       *[other] Zoom auf { $rest } setzen
    }
wm-action-debug = Debug { $args }
wm-action-casttarget = { $first ->
        [window] Ein Fenster teilen
        [output] Einen Bildschirm teilen
        [clear] Teilen beenden
       *[other] { $args } teilen
    }
wm-action-focuslast = Das vorherige Fenster fokussieren
wm-action-focusurgent = Das Fenster fokussieren, das Aufmerksamkeit verlangt
wm-action-cycle = { $first ->
        [next] Zum nächsten Fenster wechseln
        [prev] Zum vorherigen Fenster wechseln
       *[other] Fenster durchschalten: { $args }
    }
wm-action-switcher = { $first ->
        [next] Zum nächsten Fenster umschalten
        [prev] Zum vorherigen Fenster umschalten
       *[other] Fenster umschalten: { $args }
    }
wm-binds-unknown-action = Unbekannte Aktion
    .description = lys-wm kennt keine Aktion mit diesem Namen; das Tastenkürzel wird wie geschrieben angezeigt
wm-binds-category-windows = Fenster
wm-binds-category-workspaces = Arbeitsflächen
wm-binds-category-layout = Layout
wm-binds-category-apps = Apps
wm-binds-category-screenshots = Bildschirmfotos
wm-binds-category-media = Medien
wm-binds-category-system = System
wm-binds-category-custom = Eigene
wm-binds-search = Tastenkürzel, Aktionen oder Tasten suchen
## The arguments of a lys-wm action, by the form lys-wm describes
wm-action-pick = Aktion wählen
wm-action-args-outside = Keine Form dieser Aktion
    .description = lys-wm beschreibt die Argumente dieser Aktion anders; bearbeite sie als Text
enum-wm-arg-default = Standard
enum-wm-arg-all = Alle
enum-wm-arg-app = Fokussierte App
enum-wm-arg-app_id = App-ID
enum-wm-arg-app_id-colon-id = App und Tastenkürzel
enum-wm-arg-bottom = Untere Hälfte
enum-wm-arg-bottomleft = Unten links
enum-wm-arg-bottomright = Unten rechts
enum-wm-arg-center = Mitte
enum-wm-arg-clear = Nichts
enum-wm-arg-command = Befehl
enum-wm-arg-down = Unten
enum-wm-arg-dx = Breitenänderung (px)
enum-wm-arg-dy = Höhenänderung (px)
enum-wm-arg-e-minus-n = Arbeitsfläche davor öffnen, beliebiger Bildschirm
enum-wm-arg-e-plus-n = Arbeitsfläche danach öffnen, beliebiger Bildschirm
enum-wm-arg-empty = Erste leere
enum-wm-arg-ephemeral = Auflösen, wenn ein Fenster übrig ist
enum-wm-arg-factor = Faktor
enum-wm-arg-fake = Nur in der Kachel
enum-wm-arg-fill = Füllen
enum-wm-arg-follow = Dem Fenster folgen
enum-wm-arg-h = Nebeneinander
enum-wm-arg-hide = Ausblenden
enum-wm-arg-id = Fenster-ID
enum-wm-arg-in = Hinein
enum-wm-arg-key = Eigenschaft
enum-wm-arg-left = Links
enum-wm-arg-m-minus-n = Arbeitsfläche davor öffnen, dieser Bildschirm
enum-wm-arg-m-plus-n = Arbeitsfläche danach öffnen, dieser Bildschirm
enum-wm-arg-minus-n-percent = Schmaler in Prozent
enum-wm-arg-minus-n-px = Schmaler in Pixeln
enum-wm-arg-ms = Millisekunden
enum-wm-arg-n = Zahl
enum-wm-arg-n-percent = Prozent des Bildschirms
enum-wm-arg-n-px = Pixel
enum-wm-arg-name = Name
enum-wm-arg-next = Nächste
enum-wm-arg-off = Aus
enum-wm-arg-on = An
enum-wm-arg-opposite = Andersherum
enum-wm-arg-out = Heraus
enum-wm-arg-output = Bildschirm
enum-wm-arg-output-name = Bildschirm
enum-wm-arg-plus-n-percent = Breiter in Prozent
enum-wm-arg-plus-n-px = Breiter in Pixeln
enum-wm-arg-preset-next = Nächste Voreinstellung
enum-wm-arg-preset-prev = Vorherige Voreinstellung
enum-wm-arg-prev = Vorherige
enum-wm-arg-previous = Vorherige
enum-wm-arg-reset = Zurücksetzen
enum-wm-arg-restore = Alter Platz
enum-wm-arg-right = Rechts
enum-wm-arg-scroll = Scrollend
enum-wm-arg-set = Festlegen
enum-wm-arg-show = Anzeigen
enum-wm-arg-stack = Gestapelt
enum-wm-arg-tab = Reiter
enum-wm-arg-toggle = Umschalten
enum-wm-arg-toggle-name = Umschalten
enum-wm-arg-toggletab = Reiter umschalten
enum-wm-arg-top = Obere Hälfte
enum-wm-arg-topleft = Oben links
enum-wm-arg-topright = Oben rechts
enum-wm-arg-tree = Kacheln
enum-wm-arg-up = Oben
enum-wm-arg-v = Gestapelt
enum-wm-arg-value = Wert
enum-wm-arg-window = Fenster
enum-wm-arg-window-id = Fenster
enum-wm-arg-workspace = Arbeitsfläche
enum-wm-arg-wrap = Umlaufend
wm-binds-clash-mode = { $label } im Modus { $mode }
wm-binds-reset = Zurück zum Tastenkürzel von lys-wm
wm-mode = Modus { $mode }
wm-mode-enter = Tastenkürzel, das Modus { $mode } betritt
wm-mode-rename = Modus umbenennen
wm-mode-rename-confirm = Umbenennen
wm-mode-delete-button = Modus löschen
wm-mode-delete-title = Modus { $mode } löschen?
wm-mode-delete-description = Seine Tastenkürzel und die Tastenkürzel, die ihn betreten, werden ebenfalls entfernt.
wm-mode-delete-confirm = Löschen
wm-mode-add = Modus hinzufügen
wm-mode-add-confirm = Hinzufügen
wm-mode-name = Name
wm-mode-name-placeholder = Name, etwa resize
wm-mode-chord = Tastenkürzel, das den Modus betritt
wm-mode-name-missing = Ein Modus braucht einen Namen
wm-mode-name-reset = Mit { $name } verlässt ein Tastenkürzel einen Modus, daher kann es keinen benennen
wm-mode-name-taken = Ein Modus namens { $name } existiert bereits
wm-mode-chord-missing = Nimm das Tastenkürzel auf, das den Modus betritt
wm-mode-refused = lys-wm nimmt diese Änderung nicht an
wm-mode-not-saved = Die Änderung wurde nicht gespeichert
wm-mode-delete-failed = Der Modus ließ sich nicht löschen
wm-mode-unreachable = Kein Tastenkürzel betritt diesen Modus; nimm eines neben seinem Namen auf
wm-section-refraction = Lichtbrechung
    .description = Unscharfes Glas, das den Hintergrund an seinen Rändern bricht
wm-section-specular = Randglanz
    .description = Ein Glanzlicht am Rand von unscharfem Glas, auf der Seite zum Zeiger hin
wm-decoration-blur-refraction = Lichtbrechung
    .description = Den Hintergrund an den Rändern unscharfer Fenster brechen
wm-decoration-blur-refraction-bezel = Brechungsrand
    .description = Wie weit vom Rand nach innen der Hintergrund gebrochen wird, in Pixeln
wm-decoration-blur-refraction-amount = Brechungsstärke
    .description = Wie weit das Glas über den Rand hinaus blickt, in Pixeln
wm-decoration-blur-specular = Randglanz
    .description = Den Rand unscharfer Fenster aufhellen
wm-decoration-blur-specular-strength = Glanzhelligkeit
    .description = Wie hell der Rand glänzt
wm-decoration-blur-specular-width = Glanzbreite
    .description = In Pixeln
wm-scroll-presets = Spaltenbreiten
    .description = Die Breiten, durch die das Tastenkürzel für die Spaltenbreite im scrollenden Layout schaltet, in dieser Reihenfolge
    .keywords = Vorgaben, columnwidth, scrollen
wm-scroll-presets-share = Anteil des Bildschirms
wm-scroll-presets-pixels = Pixel
wm-scroll-presets-add = Breite hinzufügen
wm-scroll-presets-add-width = Eine { $width } breite Spalte hinzufügen
wm-scroll-presets-remove = Diese Breite entfernen
wm-section-touch-gestures = Touchscreen-Gesten
    .description = Wischgesten auf einem Touchscreen und was sie tun
wm-gestures-touch = Wischgesten
    .description = Vom Rand hinein wischen, oder mit drei oder mehr Fingern irgendwo auf dem Bildschirm
wm-gestures-touch-add = Wischgeste hinzufügen
wm-gestures-touch-start = Beginnt
wm-gestures-touch-does = Bewirkt
wm-gestures-touch-fingers-anywhere = Finger irgendwo
wm-gestures-touch-fingers = Finger
wm-gestures-touch-direction = Richtung
wm-gestures-touch-run = Aktion ausführen
wm-gestures-touch-follow = Den Fingern folgen
wm-gestures-touch-edge-title = { $edge ->
        [top] Vom oberen Rand hinein wischen
        [bottom] Vom unteren Rand hinein wischen
        [left] Vom linken Rand hinein wischen
        [right] Vom rechten Rand hinein wischen
       *[other] Von einem Rand hinein wischen
    }
wm-gestures-touch-fingers-title = { $direction ->
        [left] Mit { $fingers } Fingern nach links wischen
        [right] Mit { $fingers } Fingern nach rechts wischen
        [up] Mit { $fingers } Fingern nach oben wischen
        [down] Mit { $fingers } Fingern nach unten wischen
        [horizontal] Mit { $fingers } Fingern seitwärts wischen
        [vertical] Mit { $fingers } Fingern nach oben oder unten wischen
        [any] Mit { $fingers } Fingern in jede Richtung wischen
       *[other] Mit { $fingers } Fingern wischen
    }
enum-wm-edge-top = Oberer Rand
enum-wm-edge-bottom = Unterer Rand
enum-wm-edge-left = Linker Rand
enum-wm-edge-right = Rechter Rand
enum-wm-direction-left = Links
enum-wm-direction-right = Rechts
enum-wm-direction-up = Oben
enum-wm-direction-down = Unten
enum-wm-direction-horizontal = Seitwärts
enum-wm-direction-vertical = Oben oder unten
enum-wm-direction-any = Jede Richtung
enum-wm-drive-workspaces = Arbeitsflächen
enum-wm-drive-overview = Übersicht
enum-wm-drive-view = Spalten scrollen
enum-wm-drive-move = Fenster verschieben
enum-wm-drive-resize = Fenstergröße ändern
wm-input-pointer-scroll-button = Bildlauftaste
    .description = Die Taste, die beim Bewegen des Geräts gehalten wird, um zu scrollen
wm-input-touchpad-scroll-button = Bildlauftaste
    .description = Die Taste, die beim Bewegen des Geräts gehalten wird, um zu scrollen
wm-input-trackpoint-scroll-button = Bildlauftaste
    .description = Die Taste, die beim Bewegen des Geräts gehalten wird, um zu scrollen
wm-button-code = Tastencode
    .description = Eine andere Taste, über ihren evdev-Code (BTN_SIDE ist 275)
enum-wm-scroll-button-back = Zurück
enum-wm-scroll-button-extra = Extra
enum-wm-scroll-button-forward = Vorwärts
enum-wm-scroll-button-left = Links
enum-wm-scroll-button-middle = Mitte
enum-wm-scroll-button-right = Rechts
enum-wm-scroll-button-side = Seite
enum-wm-scroll-button-task = Aufgabe
wm-preview-tabs = Vorschau der Tableiste
wm-preview-tab-active = Fokussiert
wm-preview-tab-inactive = Anderes Fenster
wm-preview-tab-urgent = Will Aufmerksamkeit
wm-tabs-font-built-in = Die eigene Schrift von lys-wm
wm-tabs-font-unresolved = Keine Schriftdatei von { $family } gefunden
wm-tabs-font-unreadable = lys-wm kann die Schriftdatei von { $family } nicht lesen
wm-tabs-font-file-unreadable = Die Schriftdatei kann nicht gelesen werden
wm-section-workspaces = Arbeitsflächen
    .description = Regeln für einzelne Arbeitsflächen oder für jede passende Arbeitsfläche
wm-section-scratchpads = Notizfenster
    .description = Benannte Fenster, die per Tastenkürzel über den anderen ein- und ausgeblendet werden
wm-workspaces = Arbeitsflächenregeln
    .description = Eine Arbeitsfläche nach Nummer oder jede passende, wie sie aussieht und startet
wm-workspace-number = Arbeitsfläche { $number }
wm-workspaces-id = Nummer
wm-workspaces-match = Jede Arbeitsfläche
wm-workspaces-name = Name
wm-workspaces-output = Bildschirm
wm-workspaces-default = Zuerst auf ihrem Bildschirm gezeigt
wm-workspaces-persistent = Auch leer behalten
wm-workspaces-on-created-empty = App beim leeren Anlegen öffnen
wm-workspaces-layout = Anordnung
wm-workspaces-gaps-inner = Abstand zwischen Fenstern
wm-workspaces-gaps-outer = Abstand zum Bildschirmrand
wm-workspaces-borders-width = Rahmenbreite
wm-workspaces-rounding = Eckenrundung
wm-workspaces-shadow = Schatten
wm-scratchpads = Notizfenster
    .description = App und Größe jedes Notizfensters; ein Tastenkürzel blendet es nach Name ein oder aus
wm-scratchpads-name = Name des Notizfensters
wm-scratchpads-exec = App
wm-scratchpads-size = Größe
wm-scratchpads-hide-on-workspace-switch = Beim Wechsel der Arbeitsfläche ausblenden
wm-size-width = Breite %
wm-size-height = Höhe %
enum-wm-match-any = Jede Arbeitsfläche
enum-wm-match-single_tiled = Mit einem gekachelten Fenster
enum-wm-match-fullscreen = Mit einem Vollbildfenster
wm-section-animation-events = Animationen des Fenstermanagers
    .description = Wie sich jede Animation von lys-wm bewegt; ein leerer Wert folgt der Animation darüber
wm-section-curves = Animationskurven
    .description = Eigene Beschleunigungskurven, die Animationen über ihren Namen verwenden
wm-animations-events = Animationen
    .description = Ob jede Animation läuft, ihre Kurve, ihre Länge und ihr Stil
wm-animations-curves = Kurven
    .description = Je eine gezeichnete Kurve oder Feder, benannt für die Animationen
wm-animations-curves-name = Name der Kurve
wm-animations-curves-shape = Form
wm-curve-kind-bezier = Kurve
wm-curve-kind-spring = Feder
wm-curve-canvas = Form der Kurve; die beiden Griffe ziehen, um sie zu ändern
wm-curve-x1 = Erster Griff, Zeit
wm-curve-y1 = Erster Griff, Fortschritt
wm-curve-x2 = Zweiter Griff, Zeit
wm-curve-y2 = Zweiter Griff, Fortschritt
wm-curve-stiffness = Steifigkeit
wm-curve-damping = Dämpfung
wm-curve-mass = Masse
wm-curve-presets = Ausgehend von
wm-accel-curve-none = Das Beschleunigungsprofil bestimmt, wie der Zeiger schneller wird
wm-accel-curve-start = Eigene Kurve zeichnen
wm-accel-curve-canvas = Zeigergeschwindigkeit über Gerätegeschwindigkeit; einen Punkt nach oben oder unten ziehen, um ihn zu ändern
wm-accel-curve-step = Gerätegeschwindigkeit zwischen den Punkten
wm-accel-curve-add = Punkt hinzufügen
wm-accel-curve-remove = Letzten Punkt entfernen
wm-accel-curve-stop = Beschleunigungsprofil verwenden
wm-curve-own = Eigene Kurven
wm-curve-linear = Linear
wm-curve-ease = Weich
wm-curve-ease_in = Weich hinein
wm-curve-ease_out = Weich heraus
wm-curve-ease_in_out = Weich hinein und heraus
wm-curve-spring = Feder
wm-anim-column-enabled = Läuft
wm-anim-column-curve = Kurve
wm-anim-column-duration = Länge (ms)
wm-anim-column-style = Stil
wm-anim-event-global = Alles
wm-anim-event-windows = Fenster
wm-anim-event-windows_move = Fenster bewegen
wm-anim-event-windows_in = Fenster öffnen
wm-anim-event-windows_out = Fenster schließen
wm-anim-event-windows_minimize = Fenster minimieren
wm-anim-event-snap = Fenster einrasten
wm-anim-event-fade = Überblendungen
wm-anim-event-fade_in = Einblenden
wm-anim-event-fade_out = Ausblenden
wm-anim-event-fade_switch = Fokuswechsel
wm-anim-event-fade_dim = Abdunkeln
wm-anim-event-fade_shadow = Schatten
wm-anim-event-fade_snap = Einrastvorschau
wm-anim-event-workspaces = Arbeitsfläche wechseln
wm-anim-event-layers = Leisten und Überlagerungen
wm-anim-event-layers_in = Leisten erscheinen
wm-anim-event-layers_out = Leisten verschwinden
wm-anim-event-borderangle = Rahmenverlauf
wm-anim-event-zoom = Zoom
wm-anim-event-overview = Übersicht
wm-anim-event-scratchpad = Notizfenster
wm-anim-style-popin = Aufploppen
wm-anim-style-slide = Schieben
wm-anim-style-slidevert = Senkrecht schieben
wm-anim-style-fade = Überblenden
wm-anim-style-once = Einmal
wm-anim-style-loop = Endlos
wm-anim-style-morph = Verformen
wm-anim-style-elastic = Elastisch
wm-anim-style-goo = Zähflüssig
wm-anim-style-bud = Knospe
wm-anim-style-genie = Flaschengeist
wm-app-shortcuts-unreadable = Die registrierten Kurzbefehle lassen sich nicht lesen
wm-devices-reading = lys-wm wird gefragt, welche Geräte verbunden sind…
