lock-incorrect = Password errata
lock-failed-attempts = { $count ->
    [one] Password errata: { $count } tentativo fallito
    [many] Password errata: { $count } di tentativi falliti
   *[other] Password errata: { $count } tentativi falliti
}
lock-locked-out = Troppi tentativi falliti
lock-caps-lock = Bloc Maiusc è attivo
lock-password = Inserisci la password
lock-fingerprint-no-match = Impronta non riconosciuta
lock-fingerprint-retry = Riprova con il dito
lock-fingerprint-touch = Tocca il lettore o digita la password
lock-media-play = Riproduci
lock-media-pause = Pausa
lock-media-next = Traccia successiva
lock-battery = { $percent }%
lock-battery-charging = In carica
lock-switch-user = Cambia utente
