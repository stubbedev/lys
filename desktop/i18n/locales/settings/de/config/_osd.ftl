
settings-osd-enabled = Aktiviert
    .description = Bildschirmeinblendungen für Lautstärke, Helligkeit usw. anzeigen

settings-osd-presets = Toast-Vorlagen
    .description = Wiederverwendbare Toasts, auslösbar mit `lys-shell toast --preset <id>`

settings-osd-position = Position
    .description = Wo auf dem Bildschirm das OSD erscheint (z. B. top-center, bottom-right)

settings-osd-duration = Dauer
    .description = Wie lange das OSD sichtbar bleibt, bevor es sich automatisch schließt (ms)

settings-osd-monitor = Monitor
    .description = Welcher Monitor das OSD zeigt: "primary" oder ein Anschluss wie "DP-1"

settings-osd-margin = Rand
    .description = Abstand vom Bildschirmrand zum OSD

settings-osd-border = Rahmen
    .description = Einen Rahmen um das OSD zeichnen

settings-osd-layer = Ebene
    .description = Layer-Shell-Ebene, auf der das OSD liegt. Der Tearing-Modus stuft overlay auf top zurück.

settings-osd-text-align = Textausrichtung
    .description = Horizontale Ausrichtung des Inhalts von OSD-Toasts/-Umschaltern

settings-osd-media = Aktuelle Wiedergabe
    .description = Titelwechsel, die das OSD der aktuellen Wiedergabe zeigen: angefordert (Medientasten und `lys-shell media next`), immer oder nie


## OsdPosition variants
enum-osd-position-top-left = Oben links
enum-osd-position-top = Oben
enum-osd-position-top-right = Oben rechts
enum-osd-position-right = Rechts
enum-osd-position-bottom-right = Unten rechts
enum-osd-position-bottom = Unten
enum-osd-position-bottom-left = Unten links
enum-osd-position-left = Links


## OsdTextAlign variants
enum-osd-text-align-start = Anfang
enum-osd-text-align-center = Mitte
enum-osd-text-align-end = Ende


## OsdMedia variants
enum-osd-media-requested = Angefordert
enum-osd-media-always = Immer
enum-osd-media-never = Nie
