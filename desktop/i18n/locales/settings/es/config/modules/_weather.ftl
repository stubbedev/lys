### lys-shell Configuration - Weather Module

## Weather Module Configuration

settings-modules-weather-provider = Proveedor
    .description = Proveedor de datos meteorológicos

settings-modules-weather-location = Ubicación
    .description = La ciudad para la que es el pronóstico

settings-modules-weather-units = Unidad de temperatura
    .description = Métrico (Celsius) o imperial (Fahrenheit)

settings-modules-weather-format = Formato de la etiqueta
    .description = Lo que muestra la etiqueta, compuesto por los valores del elemento y texto propio

settings-modules-weather-refresh-interval = Intervalo de actualización
    .description = Intervalo de sondeo en segundos

settings-modules-weather-visual-crossing-key = Clave de Visual Crossing
    .description = Clave de API de Visual Crossing. Use $VAR_NAME para hacer referencia a variables de .env

settings-modules-weather-weatherapi-key = Clave de WeatherAPI
    .description = Clave de API de WeatherAPI.com. Use $VAR_NAME para hacer referencia a variables de .env

settings-modules-weather-icon-name = Icono de reserva
    .description = Icono que se muestra cuando no hay datos meteorológicos

settings-modules-weather-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-weather-border-color = Color del borde
    .description = Token de color del borde

settings-modules-weather-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-weather-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-weather-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-weather-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-weather-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta de temperatura

settings-modules-weather-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-weather-label-max-length = Longitud máxima de la etiqueta
    .description = Caracteres que muestra la etiqueta antes de recortarse; 0 no la recorta nunca

settings-modules-weather-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-weather-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-weather-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-weather-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-weather-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

settings-modules-weather-time-format = Formato de la hora
    .description = Formato de visualización de la hora en las previsiones

settings-modules-weather-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo

settings-modules-weather-refresh-interval-seconds = Intervalo de actualización
    .description = Con qué frecuencia se obtienen los datos meteorológicos (segundos)

## Format placeholders

settings-modules-weather-format-placeholder-temp = Temperatura actual

settings-modules-weather-format-placeholder-temp-unit = Símbolo de la unidad de temperatura (°F o °C)

settings-modules-weather-format-placeholder-feels-like = Sensación térmica

settings-modules-weather-format-placeholder-condition = Estado del tiempo (p. ej. Nublado)

settings-modules-weather-format-placeholder-humidity = Porcentaje de humedad (p. ej. 65%)

settings-modules-weather-format-placeholder-wind-speed = Velocidad del viento con su unidad (p. ej. 12 km/h)

settings-modules-weather-format-placeholder-wind-dir = Dirección del viento (p. ej. NO)

settings-modules-weather-format-placeholder-high = Temperatura máxima de hoy (vacío sin previsión)

settings-modules-weather-format-placeholder-low = Temperatura mínima de hoy (vacío sin previsión)
