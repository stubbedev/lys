### lys-shell Configuration - Weather Module

## Weather Module Configuration

settings-modules-weather-provider = Anbieter
    .description = Anbieter der Wetterdaten

settings-modules-weather-location = Ort
    .description = Die Stadt, für die die Vorhersage gilt

settings-modules-weather-units = Temperatureinheit
    .description = Metrisch (Celsius) oder Imperial (Fahrenheit)

settings-modules-weather-format = Beschriftungsformat
    .description = Formatzeichenkette mit Platzhaltern: {"{{ temp }}"}, {"{{ temp_unit }}"}, {"{{ feels_like }}"}, {"{{ condition }}"}, {"{{ humidity }}"}, {"{{ wind_speed }}"}, {"{{ wind_dir }}"}, {"{{ high }}"}, {"{{ low }}"}

settings-modules-weather-refresh-interval = Aktualisierungsintervall
    .description = Abfrageintervall in Sekunden

settings-modules-weather-visual-crossing-key = Visual-Crossing-Schlüssel
    .description = API-Schlüssel für Visual Crossing. Mit $VAR_NAME auf .env-Variablen verweisen

settings-modules-weather-weatherapi-key = WeatherAPI-Schlüssel
    .description = API-Schlüssel für WeatherAPI.com. Mit $VAR_NAME auf .env-Variablen verweisen

settings-modules-weather-icon-name = Ersatzsymbol
    .description = Symbol, wenn keine Wetterdaten verfügbar sind

settings-modules-weather-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-weather-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-weather-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-weather-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-weather-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-weather-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-weather-label-show = Beschriftung anzeigen
    .description = Temperaturbeschriftung anzeigen

settings-modules-weather-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-weather-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-weather-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-weather-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-weather-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-weather-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-weather-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

settings-modules-weather-time-format = Uhrzeitformat
    .description = Anzeigeformat der Uhrzeit in Vorhersagen

settings-modules-weather-left-click = Linksklick
    .description = Aktion bei Linksklick

settings-modules-weather-refresh-interval-seconds = Aktualisierungsintervall
    .description = Wie oft Wetterdaten abgerufen werden (Sekunden)

## Format placeholders

settings-modules-weather-format-placeholder-temp = Aktuelle Temperatur

settings-modules-weather-format-placeholder-temp-unit = Symbol der Temperatureinheit (°F oder °C)

settings-modules-weather-format-placeholder-feels-like = Gefühlte Temperatur

settings-modules-weather-format-placeholder-condition = Wetterlage (z. B. Bewölkt)

settings-modules-weather-format-placeholder-humidity = Luftfeuchtigkeit in Prozent (z. B. 65%)

settings-modules-weather-format-placeholder-wind-speed = Windgeschwindigkeit mit Einheit (z. B. 12 km/h)

settings-modules-weather-format-placeholder-wind-dir = Windrichtung (z. B. NW)

settings-modules-weather-format-placeholder-high = Heutige Höchsttemperatur (leer ohne Vorhersage)

settings-modules-weather-format-placeholder-low = Heutige Tiefsttemperatur (leer ohne Vorhersage)
