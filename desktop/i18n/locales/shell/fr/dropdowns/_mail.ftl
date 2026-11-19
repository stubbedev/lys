### Menu déroulant du courrier

dropdown-mail-title = Courrier
dropdown-mail-unread = Courrier non lu
dropdown-mail-error-title = Impossible de compter le courrier non lu
dropdown-mail-account-error = Comptage impossible
mail-notification-sender = Nouveau courrier
mail-notification-arrived = { $new ->
    [one] { $new } nouveau message
    [many] { $new } de nouveaux messages
   *[other] { $new } nouveaux messages
} ({ $count ->
    [one] { $count } non lu
    [many] { $count } non lus
   *[other] { $count } non lus
})
mail-notification-open = Ouvrir
