### Mail Dropdown

dropdown-mail-title = Correo
dropdown-mail-unread = Correo sin leer
dropdown-mail-error-title = No se ha podido contar el correo sin leer
dropdown-mail-account-error = No se ha podido contar
mail-notification-sender = Correo nuevo
mail-notification-arrived = { $new ->
    [one] { $new } mensaje nuevo
    [many] { $new } de mensajes nuevos
   *[other] { $new } mensajes nuevos
} ({ $count ->
    [one] { $count } sin leer
    [many] { $count } sin leer
   *[other] { $count } sin leer
})
