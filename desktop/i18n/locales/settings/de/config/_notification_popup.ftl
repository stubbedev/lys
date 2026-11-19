### lys-shell Configuration - Notification Module Settings

## Shared Settings

settings-modules-notifications-blocklist = Sperrliste
    .description = Glob-Muster für App-Namen, deren Benachrichtigungen blockiert werden

settings-modules-notifications-icon-source = Symbolquelle
    .description = Wie Benachrichtigungssymbole ermittelt werden

settings-modules-notifications-history-memory = Verlauf im Speicher
    .description = Neueste Benachrichtigungen, die im Speicher bleiben; ältere werden beim Scrollen von der Festplatte geladen

settings-modules-notifications-history-stored = Verlauf auf der Festplatte
    .description = Auf der Festplatte gespeicherte Benachrichtigungen; darüber hinaus werden die ältesten gelöscht

## Popup Settings

settings-modules-notifications-popup-position = Popup-Position
    .description = Bildschirmecke, in der Benachrichtigungs-Popups erscheinen

settings-modules-notifications-popup-max-visible = Maximal sichtbar
    .description = Maximale Anzahl gleichzeitig angezeigter Popups, bevor gestapelt wird

settings-modules-notifications-popup-stacking-order = Stapelreihenfolge
    .description = Ob neue Popups über oder unter den vorhandenen erscheinen

settings-modules-notifications-popup-stack = Stapeln
    .description = Welche Popups zu einem Stapel zusammengefasst werden, der sich beim Überfahren auffächert

settings-modules-notifications-popup-opacity = Ruhedeckkraft
    .description = Deckkraft der Milchglasfläche eines Popups, solange der Zeiger woanders ist

settings-modules-notifications-popup-margin-x = Horizontaler Rand
    .description = Abstand zum linken/rechten Bildschirmrand

settings-modules-notifications-popup-margin-y = Vertikaler Rand
    .description = Abstand zum oberen/unteren Bildschirmrand

settings-modules-notifications-popup-gap = Popup-Abstand
    .description = Abstand zwischen gestapelten Benachrichtigungs-Popups

settings-modules-notifications-popup-monitor = Popup-Monitor
    .description = Welcher Monitor Popups zeigt: "primary" oder ein Anschluss wie "DP-1"

settings-modules-notifications-popup-layer = Popup-Ebene
    .description = Layer-Shell-Ebene, auf der Benachrichtigungs-Popups liegen. Der Tearing-Modus stuft overlay auf top zurück.

settings-modules-notifications-popup-duration = Automatisch schließen
    .description = Wie lange Popups sichtbar bleiben, bevor sie sich schließen

settings-modules-notifications-popup-hover-pause = Beim Überfahren pausieren
    .description = Den Timer zum automatischen Schließen anhalten, solange der Zeiger über einem Popup ist

settings-modules-notifications-popup-close-behavior = Schließverhalten
    .description = Was beim Schließen eines Popups passiert (verwerfen oder als gelesen markieren)

settings-modules-notifications-popup-urgency-bar = Dringlichkeitsbalken
    .description = Einen farbigen Balken auf Popups anzeigen, die eine Mindestdringlichkeit erreichen

settings-modules-notifications-sounds = Töne
    .description = Welche Benachrichtigungen einen Ton abspielen; stumm bei „Nicht stören“, während der Bildschirmfreigabe und bei Stummschaltung

settings-modules-notifications-sound-default = Standardton
    .description = Name aus dem Klangthema, den eine Benachrichtigung ohne eigenen Ton abspielt

settings-modules-notifications-sound-mute-apps = Stumme Apps
    .description = Glob-Muster für App-Namen, deren Benachrichtigungen nie einen Ton abspielen
settings-modules-notifications-sound-volume = Tonlautstärke
    .description = Wie laut Benachrichtigungstöne abgespielt werden, als Anteil der aufgenommenen Lautstärke
