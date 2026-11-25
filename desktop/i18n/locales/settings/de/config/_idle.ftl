### lys-shell Configuration - Idle

## Idle Configuration

settings-idle-enable = Eingebaute Leerlaufrichtlinie
    .description = Bei Leerlauf abdunkeln, sperren, Bildschirme ausschalten und in Bereitschaft wechseln; ausschalten, wenn swayidle oder hypridle den Leerlauf übernimmt

settings-idle-dim-percent = Abgedunkelte Helligkeit
    .description = Anteil ihrer Helligkeit in Prozent, den eine abgedunkelte Hintergrundbeleuchtung behält

settings-idle-suspend-action = Schlafmodus
    .description = Wie das System schläft, wenn die Bereitschaftsstufe greift, über logind

settings-idle-lock-before-sleep = Vor Bereitschaft sperren
    .description = Die Sitzung sperren, bevor das System in Bereitschaft geht, damit es immer gesperrt aufwacht

settings-idle-timeouts-ac = Im Netzbetrieb
    .description = Leerlaufzeit als Dauer wie 90s, 5m oder 1h30m, oder never

settings-idle-timeouts-battery = Im Akkubetrieb
    .description = Leerlaufzeit als Dauer wie 90s, 5m oder 1h30m, oder never

settings-idle-attention = Vor dem Abdunkeln hinsehen
    .description = Bevor der Bildschirm abdunkelt, die Kamera für weniger als eine Sekunde einschalten, zuerst die Infrarotkamera, um zu sehen, ob ein Gesicht dem Bildschirm zugewandt ist: Ein Gesicht schiebt das Abdunkeln auf. Ihre Leuchte geht dabei an. Das Bild wird nur auf ein Gesicht geprüft und verworfen, nie gespeichert, protokolliert oder gesendet, und nichts wird angesehen, solange die Sitzung gesperrt ist oder eine andere App die Kamera nutzt

## SleepAction variants
enum-sleep-action-auto = Automatisch
enum-sleep-action-suspend = Bereitschaft
enum-sleep-action-hibernate = Ruhezustand
enum-sleep-action-hybrid-sleep = Hybrider Schlaf
enum-sleep-action-suspend-then-hibernate = Bereitschaft, dann Ruhezustand

## IdleAttention variants
enum-idle-attention-off = Aus
enum-idle-attention-postpone-dim = Nur das Abdunkeln aufschieben
enum-idle-attention-sooner-when-absent = Bei Abwesenheit auch den Bildschirm früher ausschalten
