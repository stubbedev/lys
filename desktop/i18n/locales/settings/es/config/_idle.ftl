### lys-shell Configuration - Idle

## Idle Configuration

settings-idle-enable = Política de inactividad integrada
    .description = Atenúa, bloquea, apaga las pantallas y suspende en inactividad; desactívela si swayidle o hypridle gestionan la inactividad

settings-idle-dim-percent = Brillo atenuado
    .description = Porcentaje de su nivel que conserva una retroiluminación atenuada

settings-idle-suspend-action = Modo de reposo
    .description = Cómo duerme el sistema cuando llega la etapa de suspensión, a través de logind

settings-idle-lock-before-sleep = Bloquear antes de suspender
    .description = Bloquea la sesión antes de que el sistema se suspenda, para que siempre se reanude bloqueada

settings-idle-timeouts-ac = Con corriente alterna
    .description = Tiempo de inactividad como una duración, por ejemplo 90s, 5m o 1h30m, o never

settings-idle-timeouts-battery = Con batería
    .description = Tiempo de inactividad como una duración, por ejemplo 90s, 5m o 1h30m, o never

settings-idle-attention = Mirar antes de atenuar
    .description = Antes de que la pantalla se atenúe, encender la cámara menos de un segundo, primero la de infrarrojos, para ver si hay una cara mirando a la pantalla: una cara aplaza la atenuación. Su luz se enciende mientras mira. La imagen solo se examina en busca de una cara y se descarta, nunca se guarda, registra ni envía, y no se mira nada mientras la sesión está bloqueada u otra aplicación usa la cámara

## SleepAction variants
enum-sleep-action-auto = Suspender y luego hibernar si se puede
enum-sleep-action-suspend = Suspender
enum-sleep-action-hibernate = Hibernar
enum-sleep-action-hybrid-sleep = Suspensión híbrida
enum-sleep-action-suspend-then-hibernate = Suspender y luego hibernar

## IdleAttention variants
enum-idle-attention-off = Desactivado
enum-idle-attention-postpone-dim = Solo aplazar la atenuación
enum-idle-attention-sooner-when-absent = Apagar también antes la pantalla cuando no hay nadie
