### lys-shell Configuration - Storage Module

## Storage Module Configuration

settings-modules-storage-poll-interval-ms = Abfrageintervall
    .description = Abfrageintervall in Millisekunden

settings-modules-storage-mount-point = Einhängepunkt(e)
    .description = Zu überwachende Einhängepunkte (z. B. "/" oder ["/", "/mnt/drive1"])

settings-modules-storage-format = Anzeigeformat
    .description = Formatzeichenkette mit Platzhaltern: {"{{ percent }}"}, {"{{ used_tib }}"}, {"{{ used_gib }}"}, {"{{ used_mib }}"}, {"{{ used_auto }}"}, {"{{ total_tib }}"}, {"{{ total_gib }}"}, {"{{ total_mib }}"}, {"{{ total_auto }}"}, {"{{ free_tib }}"}, {"{{ free_gib }}"}, {"{{ free_mib }}"}, {"{{ free_auto }}"}, {"{{ filesystem }}"}

settings-modules-storage-icon-name = Symbolname
    .description = Symbolischer Symbolname

settings-modules-storage-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-storage-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-storage-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-storage-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-storage-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-storage-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-storage-label-show = Beschriftung anzeigen
    .description = Textbeschriftung anzeigen

settings-modules-storage-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-storage-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-storage-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-storage-left-click = Linksklick
    .description = Shell-Befehl bei Linksklick

settings-modules-storage-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-storage-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-storage-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-storage-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

settings-modules-storage-thresholds = Schwellenwerte
    .description = Farbschwellen, die sich nach dem Wert ändern

## Format placeholders

settings-modules-storage-format-placeholder-percent = Datenträgerauslastung (0–100)

settings-modules-storage-format-placeholder-used-tib = Belegter Platz in TiB

settings-modules-storage-format-placeholder-used-gib = Belegter Platz in GiB

settings-modules-storage-format-placeholder-used-mib = Belegter Platz in MiB

settings-modules-storage-format-placeholder-used-auto = Belegter Platz mit Einheit (z. B. 128.5 GiB)

settings-modules-storage-format-placeholder-total-tib = Gesamter Platz in TiB

settings-modules-storage-format-placeholder-total-gib = Gesamter Platz in GiB

settings-modules-storage-format-placeholder-total-mib = Gesamter Platz in MiB

settings-modules-storage-format-placeholder-total-auto = Gesamter Platz mit Einheit

settings-modules-storage-format-placeholder-free-tib = Freier Platz in TiB

settings-modules-storage-format-placeholder-free-gib = Freier Platz in GiB

settings-modules-storage-format-placeholder-free-mib = Freier Platz in MiB

settings-modules-storage-format-placeholder-free-auto = Freier Platz mit Einheit

settings-modules-storage-format-placeholder-filesystem = Dateisystemtyp (z. B. ext4, btrfs)
