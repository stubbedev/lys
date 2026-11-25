### lys-shell Configuration - Idle

## Idle Configuration

settings-idle-enable = Built-in idle policy
    .description = Dim, lock, power the screens off and suspend when idle; turn off when swayidle or hypridle handles idle

settings-idle-dim-percent = Dimmed brightness
    .description = Share of its level, in percent, a dimmed backlight keeps

settings-idle-suspend-action = Sleep
    .description = How the system sleeps when the suspend stage runs, through logind

settings-idle-lock-before-sleep = Lock before sleep
    .description = Lock the session before the system sleeps, so it always wakes locked

settings-idle-timeouts-ac = On AC power
    .description = Idle time as a duration such as 90s, 5m or 1h30m, or never

settings-idle-timeouts-battery = On battery
    .description = Idle time as a duration such as 90s, 5m or 1h30m, or never

settings-idle-attention = Look before dimming
    .description = Before the screen dims, switch the camera on for under a second, the infrared one first, to see whether a face is turned to the screen: a face postpones the dim. Its light comes on while it looks. The picture is only checked for a face and dropped, never saved, logged or sent, and nothing is looked at while the session is locked or another app uses the camera

## SleepAction variants
enum-sleep-action-auto = Automatic
enum-sleep-action-suspend = Suspend
enum-sleep-action-hibernate = Hibernate
enum-sleep-action-hybrid-sleep = Hybrid sleep
enum-sleep-action-suspend-then-hibernate = Suspend, then hibernate

## IdleAttention variants
enum-idle-attention-off = Off
enum-idle-attention-postpone-dim = Postpone dimming only
enum-idle-attention-sooner-when-absent = Also turn the screen off sooner when nobody is there
