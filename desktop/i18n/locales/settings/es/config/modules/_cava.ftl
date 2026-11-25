### lys-shell Configuration - Cava Module

## Audio Processing

settings-modules-cava-bars = Número de barras
    .description = Número de barras de frecuencia

settings-modules-cava-framerate = Fotogramas por segundo
    .description = Frecuencia de actualización de la visualización en fotogramas por segundo

settings-modules-cava-stereo = Estéreo
    .description = Divide las barras entre los canales de audio izquierdo y derecho

settings-modules-cava-noise-reduction = Reducción de ruido
    .description = Intensidad del filtro de reducción de ruido (0.0 rápido/ruidoso a 1.0 lento/suave)

settings-modules-cava-monstercat = Suavizado Monstercat
    .description = Caída suave entre barras adyacentes (0.0 = desactivado)

settings-modules-cava-waves = Suavizado de onda
    .description = Número de pasadas de suavizado tipo onda (0 = desactivado)

settings-modules-cava-low-cutoff = Corte inferior
    .description = Frecuencia de corte inferior en Hz

settings-modules-cava-high-cutoff = Corte superior
    .description = Frecuencia de corte superior en Hz

settings-modules-cava-input = Método de entrada
    .description = Motor de captura de audio

settings-modules-cava-source = Fuente
    .description = El sonido que siguen las barras: elegido automáticamente, o una fuente por su nombre

## Visualization

settings-modules-cava-style = Estilo
    .description = Estilo de representación de la visualización (barras, onda o picos)

settings-modules-cava-direction = Dirección
    .description = Dirección de crecimiento de las barras respecto al borde de la barra

settings-modules-cava-color = Color
    .description = Color de la visualización

settings-modules-cava-bar-width = Anchura de las barras
    .description = Anchura de cada barra de frecuencia en píxeles

settings-modules-cava-bar-gap = Separación entre barras
    .description = Separación entre las barras de frecuencia en píxeles

## Container

settings-modules-cava-internal-padding = Relleno interno
    .description = Relleno en los extremos del visualizador en rem

settings-modules-cava-button-bg-color = Color de fondo
    .description = Color de fondo del módulo

settings-modules-cava-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del visualizador

settings-modules-cava-border-color = Color del borde
    .description = Token de color del borde

settings-modules-cava-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo

settings-modules-cava-right-click = Clic derecho
    .description = Acción al hacer clic derecho

settings-modules-cava-middle-click = Clic central
    .description = Acción al hacer clic central

settings-modules-cava-scroll-up = Desplazar hacia arriba
    .description = Acción al desplazar hacia arriba

settings-modules-cava-scroll-down = Desplazar hacia abajo
    .description = Acción al desplazar hacia abajo


## CavaStyle variants
enum-cava-style-bars = Barras
enum-cava-style-wave = Onda
enum-cava-style-peaks = Picos

## CavaDirection variants
enum-cava-direction-normal = Normal
enum-cava-direction-reverse = Inversa
enum-cava-direction-mirror = Espejo
