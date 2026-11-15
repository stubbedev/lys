### lys-shell Configuration - CPU Module

## CPU Module Configuration

settings-modules-cpu-poll-interval-ms = Abfrageintervall
    .description = Abfrageintervall in Millisekunden

settings-modules-cpu-temp-sensor = Temperatursensor
    .description = Sensorbezeichnung für Temperaturwerte (z. B. "Tctl", "auto")

settings-modules-cpu-format = Anzeigeformat
    .description = Formatzeichenkette mit Platzhaltern: {"{{ percent }}"}, {"{{ freq_ghz }}"}, {"{{ avg_freq_ghz }}"}, {"{{ max_freq_ghz }}"}, {"{{ temp_c }}"}, {"{{ temp_f }}"}

settings-modules-cpu-icon-name = Symbolname
    .description = Symbolischer Symbolname

settings-modules-cpu-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-cpu-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-cpu-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-cpu-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-cpu-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-cpu-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-cpu-label-show = Beschriftung anzeigen
    .description = Textbeschriftung anzeigen

settings-modules-cpu-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-cpu-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-cpu-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-cpu-left-click = Linksklick
    .description = Shell-Befehl bei Linksklick

settings-modules-cpu-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-cpu-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-cpu-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-cpu-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

settings-modules-cpu-thresholds = Schwellenwerte
    .description = Farbschwellen, die sich nach dem Wert ändern

## Format placeholders

settings-modules-cpu-format-placeholder-percent = CPU-Auslastung (0–100)

settings-modules-cpu-format-placeholder-freq-ghz = Frequenz des am stärksten ausgelasteten Kerns in GHz

settings-modules-cpu-format-placeholder-avg-freq-ghz = Durchschnittliche Frequenz aller Kerne in GHz

settings-modules-cpu-format-placeholder-max-freq-ghz = Höchste Frequenz unter den Kernen in GHz

settings-modules-cpu-format-placeholder-temp-c = Temperatur in Celsius (0, wenn nicht verfügbar)

settings-modules-cpu-format-placeholder-temp-f = Temperatur in Fahrenheit (32, wenn nicht verfügbar)
