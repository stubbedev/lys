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

## SleepAction variants
enum-sleep-action-suspend = Suspend
enum-sleep-action-hibernate = Hibernate
enum-sleep-action-hybrid-sleep = Hybrid sleep
enum-sleep-action-suspend-then-hibernate = Suspend, then hibernate
