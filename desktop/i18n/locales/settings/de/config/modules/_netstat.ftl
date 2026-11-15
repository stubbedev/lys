### lys-shell Configuration - Netstat Module

## Netstat Module Configuration

settings-modules-netstat-poll-interval-ms = Abfrageintervall
    .description = Abfrageintervall in Millisekunden

settings-modules-netstat-interface = Schnittstelle
    .description = Zu überwachende Netzwerkschnittstelle ("auto" für die aktive Schnittstelle)

settings-modules-netstat-format = Anzeigeformat
    .description = Formatzeichenkette mit Platzhaltern: {"{{ down_kib }}"}, {"{{ down_mib }}"}, {"{{ down_gib }}"}, {"{{ down_auto }}"}, {"{{ up_kib }}"}, {"{{ up_mib }}"}, {"{{ up_gib }}"}, {"{{ up_auto }}"}, {"{{ interface }}"}

settings-modules-netstat-icon-name = Symbolname
    .description = Symbolischer Symbolname

settings-modules-netstat-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-netstat-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-netstat-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-netstat-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-netstat-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-netstat-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-netstat-label-show = Beschriftung anzeigen
    .description = Textbeschriftung anzeigen

settings-modules-netstat-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-netstat-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-netstat-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-netstat-left-click = Linksklick
    .description = Shell-Befehl bei Linksklick

settings-modules-netstat-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-netstat-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-netstat-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-netstat-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

## Format placeholders

settings-modules-netstat-format-placeholder-down-kib = Download-Geschwindigkeit in KiB/s

settings-modules-netstat-format-placeholder-down-mib = Download-Geschwindigkeit in MiB/s

settings-modules-netstat-format-placeholder-down-gib = Download-Geschwindigkeit in GiB/s

settings-modules-netstat-format-placeholder-down-auto = Download-Geschwindigkeit mit Einheit (z. B. 1.5 MiB/s)

settings-modules-netstat-format-placeholder-up-kib = Upload-Geschwindigkeit in KiB/s

settings-modules-netstat-format-placeholder-up-mib = Upload-Geschwindigkeit in MiB/s

settings-modules-netstat-format-placeholder-up-gib = Upload-Geschwindigkeit in GiB/s

settings-modules-netstat-format-placeholder-up-auto = Upload-Geschwindigkeit mit Einheit (z. B. 256 KiB/s)

settings-modules-netstat-format-placeholder-interface = Name der Schnittstelle (z. B. wlan0)
