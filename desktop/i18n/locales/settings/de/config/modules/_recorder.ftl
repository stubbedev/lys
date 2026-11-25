### lys-shell Configuration - Recorder Module

## Recorder Module Configuration

settings-modules-recorder-icon-idle = Symbol für untätig
    .description = Symbol, wenn nicht aufgenommen wird

settings-modules-recorder-icon-recording = Aufnahmesymbol
    .description = Symbol während der Aufnahme

settings-modules-recorder-icon-paused = Symbol für pausiert
    .description = Symbol, solange die Aufnahme pausiert ist

settings-modules-recorder-icon-replay = Wiederholungssymbol
    .description = Symbol, solange nur die Sofortwiederholung bereit ist

settings-modules-recorder-format = Format
    .description = Was die Beschriftung zeigt, zusammengesetzt aus den Werten des Elements und eigenem Text

settings-modules-recorder-microphone = Mikrofon
    .description = Das Mikrofon in der Aufnahme mitschneiden

settings-modules-recorder-microphone-device = Mikrofongerät
    .description = Name der Mikrofonquelle (leer verwendet die Standardquelle)

settings-modules-recorder-system-audio = Systemaudio
    .description = Desktop-Audio in der Aufnahme mitschneiden

settings-modules-recorder-framerate = Bildrate
    .description = Aufnahme-Bildrate in Bildern pro Sekunde

settings-modules-recorder-webcam-enabled = Webcam-Rahmen
    .description = Einen Bild-in-Bild-Rahmen der Webcam in die Aufnahme einblenden

settings-modules-recorder-webcam-device = Webcam-Gerät
    .description = V4L2-Gerätepfad (leer wählt automatisch die erste Kamera)

settings-modules-recorder-webcam-x = Webcam-X-Position
    .description = Horizontale Position (0 % = links, 100 % = rechts), relativ, damit sie Auflösungswechsel übersteht

settings-modules-recorder-webcam-y = Webcam-Y-Position
    .description = Vertikale Position (0 % = oben, 100 % = unten), relativ, damit sie Auflösungswechsel übersteht

settings-modules-recorder-webcam-size = Webcam-Größe
    .description = Breite des Webcam-Rahmens in Prozent der Aufnahmebreite

settings-modules-recorder-output-directory = Ausgabeordner
    .description = Wo Aufnahmen gespeichert werden (leer verwendet den XDG-Videoordner)

settings-modules-recorder-output-format = Ausgabeformat
    .description = Containerformat / Codec-Vorlage

settings-modules-recorder-show-cursor = Zeiger anzeigen
    .description = Den Mauszeiger in der Aufnahme zeichnen

settings-modules-recorder-start-delay-ms = Startverzögerung
    .description = Verzögerung zwischen der Wahl der Quelle und der Aufnahme, damit der Start-Toast zuerst vom Bildschirm verschwindet

settings-modules-recorder-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-recorder-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-recorder-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-recorder-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-recorder-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-recorder-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-recorder-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-recorder-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-recorder-label-max-length = Maximale Beschriftungslänge
    .description = Zeichen, die die Beschriftung zeigt, bevor sie gekürzt wird; 0 kürzt nie

settings-modules-recorder-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-recorder-left-click = Linksklick
    .description = Shell-Befehl bei Linksklick

settings-modules-recorder-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-recorder-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-recorder-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-recorder-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten


## RecorderFormat variants
enum-recorder-format-mp4 = MP4
enum-recorder-format-mkv = MKV

## Instant replay

settings-modules-recorder-replay-enable = Sofortwiederholung
    .description = Die letzten Sekunden einer Ausgabe im Speicher halten, bereit zum Speichern

settings-modules-recorder-replay-seconds = Wiederholungslänge
    .description = Sekunden an Material, die beim Speichern einer Wiederholung geschrieben werden (5–600)

settings-modules-recorder-replay-output = Wiederholungsausgabe
    .description = Der aufgenommene Bildschirm: der fokussierte Monitor beim Scharfschalten der Wiederholung oder einer über seinen Anschluss, etwa DP-1

## Format placeholders

settings-modules-recorder-format-placeholder-state = Zustand der Aufnahme (Untätig, Wiederholung bereit, Aufnahme läuft, Pausiert)

settings-modules-recorder-format-placeholder-elapsed = Verstrichene Aufnahmezeit als M:SS oder H:MM:SS (- wenn untätig)
