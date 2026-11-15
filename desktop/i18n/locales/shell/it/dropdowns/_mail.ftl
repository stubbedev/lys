### Mail Dropdown

dropdown-mail-title = Posta
dropdown-mail-unread = Posta non letta
dropdown-mail-error-title = Impossibile contare la posta non letta
dropdown-mail-account-error = Impossibile contare
mail-notification-sender = Nuova posta
mail-notification-arrived = { $new ->
    [one] { $new } nuovo messaggio
    [many] { $new } di nuovi messaggi
   *[other] { $new } nuovi messaggi
} ({ $count ->
    [one] { $count } non letto
    [many] { $count } non letti
   *[other] { $count } non letti
})
