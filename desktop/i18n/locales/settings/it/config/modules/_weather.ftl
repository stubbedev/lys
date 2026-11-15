### lys-shell Configuration - Weather Module

## Weather Module Configuration

settings-modules-weather-provider = Provider
    .description = Provider dei dati meteo

settings-modules-weather-location = Località
    .description = Nome della città o coordinate "lat,lon"

settings-modules-weather-units = Unità di temperatura
    .description = Metrico (Celsius) o imperiale (Fahrenheit)

settings-modules-weather-format = Formato dell'etichetta
    .description = Stringa di formato con i segnaposto: {"{{ temp }}"}, {"{{ temp_unit }}"}, {"{{ feels_like }}"}, {"{{ condition }}"}, {"{{ humidity }}"}, {"{{ wind_speed }}"}, {"{{ wind_dir }}"}, {"{{ high }}"}, {"{{ low }}"}

settings-modules-weather-refresh-interval = Intervallo di aggiornamento
    .description = Intervallo di aggiornamento in secondi

settings-modules-weather-visual-crossing-key = Chiave Visual Crossing
    .description = Chiave API per Visual Crossing. Usa $VAR_NAME per fare riferimento alle variabili di .env

settings-modules-weather-weatherapi-key = Chiave WeatherAPI
    .description = Chiave API per WeatherAPI.com. Usa $VAR_NAME per fare riferimento alle variabili di .env

settings-modules-weather-icon-name = Icona di riserva
    .description = Icona mostrata quando i dati meteo non sono disponibili

settings-modules-weather-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-weather-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-weather-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-weather-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-weather-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-weather-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-weather-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta della temperatura

settings-modules-weather-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-weather-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-weather-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-weather-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-weather-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-weather-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-weather-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

settings-modules-weather-time-format = Formato dell'ora
    .description = Formato dell'ora per le previsioni

settings-modules-weather-left-click = Clic sinistro
    .description = Azione al clic sinistro

settings-modules-weather-refresh-interval-seconds = Intervallo di aggiornamento
    .description = Ogni quanto vengono scaricati i dati meteo (secondi)

## Format placeholders

settings-modules-weather-format-placeholder-temp = Temperatura attuale

settings-modules-weather-format-placeholder-temp-unit = Simbolo dell'unità di temperatura (°F o °C)

settings-modules-weather-format-placeholder-feels-like = Temperatura percepita

settings-modules-weather-format-placeholder-condition = Condizione meteo (ad es. Nuvoloso)

settings-modules-weather-format-placeholder-humidity = Percentuale di umidità (ad es. 65%)

settings-modules-weather-format-placeholder-wind-speed = Velocità del vento con la sua unità (ad es. 12 km/h)

settings-modules-weather-format-placeholder-wind-dir = Direzione del vento (ad es. NO)

settings-modules-weather-format-placeholder-high = Temperatura massima di oggi (vuoto senza previsioni)

settings-modules-weather-format-placeholder-low = Temperatura minima di oggi (vuoto senza previsioni)
