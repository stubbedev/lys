### Battery Dropdown

dropdown-battery-title = Batería

## Hero States
dropdown-battery-on-battery = Con batería
dropdown-battery-charging = Cargando
dropdown-battery-plugged-in = Enchufado
dropdown-battery-not-charging = Enchufado, sin cargar
dropdown-battery-critical = Crítica

## Time Display
dropdown-battery-duration-hm = { $hours } h { $minutes } min
dropdown-battery-duration-m = { $minutes } min
dropdown-battery-time-remaining = Quedan { $duration }
dropdown-battery-time-until-full = { $duration } hasta la carga completa

## Details
dropdown-battery-draw = Consumo
dropdown-battery-input = Entrada
dropdown-battery-input-watts = { $watts } de entrada
dropdown-battery-capacity = Capacidad
dropdown-battery-charged = Cargada
dropdown-battery-health = Salud

## Charge Limit
dropdown-battery-charge-limit = Límite de carga
dropdown-battery-limit-to = Limitar al { $threshold }%
dropdown-battery-charge-limit-not-supported = Este dispositivo no admite el límite de carga
dropdown-battery-health-cycles = Salud, { $cycles ->
    [one] { $cycles } ciclo
   *[other] { $cycles } ciclos
}
dropdown-battery-hold = Se mantiene al { $limit } % para proteger la batería
dropdown-battery-hold-until = Se mantiene al { $limit } % para proteger la batería; llena a las { $time }
dropdown-battery-hold-off = Cargando al 100 %: no hay límite de carga
dropdown-battery-full-requested = Cargando al 100 % hasta que desconectes
dropdown-battery-full-unplug = Cargando al 100 % para cuando sueles desconectar, llena a las { $time }
dropdown-battery-full-event = Cargando al 100 % para tu evento a las { $time }
dropdown-battery-charge-full = Cargar al máximo ahora
dropdown-battery-charge-full-failed = No se pudo empezar a cargar al máximo

## Power Profile
dropdown-battery-power-profile = Perfil de energía
dropdown-battery-profile-saver = Ahorro
dropdown-battery-profile-balanced = Equilibrado
dropdown-battery-profile-performance = Rendimiento
dropdown-battery-power-profile-not-available = El servicio de perfiles de energía debe estar activo

## No Battery
dropdown-battery-no-battery-title = No se ha detectado ninguna batería
dropdown-battery-no-battery-description = Funcionando con corriente alterna

## Peripherals
dropdown-battery-devices = Dispositivos

## Significant Energy
dropdown-battery-energy = Usan mucha energía
dropdown-battery-energy-none = Ninguna app usa mucha energía
dropdown-battery-energy-measuring = Midiendo…
dropdown-battery-energy-cpu = { $percent }% de una CPU
dropdown-battery-energy-watts = { $watts }, { $percent }% de una CPU
dropdown-battery-energy-show = Mostrar
dropdown-battery-energy-quit = Salir
dropdown-battery-energy-quit-failed = No se pudo cerrar { $app }
dropdown-battery-energy-show-failed = No se ha podido mostrar { $app }
dropdown-battery-paused = En pausa con batería
dropdown-battery-paused-now = En pausa hasta que lo conectes a la corriente
dropdown-battery-paused-failed = No se pudo pausar
dropdown-battery-paused-resume-failed = No se pudo reanudar

## Alerts
battery-alert-low = Batería baja
battery-alert-critical = Batería críticamente baja
battery-alert-remaining = Queda un { $percent }%
battery-alert-charged = Batería cargada
battery-alert-charged-body = Cargada al { $percent }%, puede desenchufar el cargador
battery-alert-device-low = Batería de { $device } baja
battery-alert-device-critical = Batería de { $device } críticamente baja
battery-offer-low-power = ¿Activar el modo de bajo consumo?
battery-offer-low-power-body = Queda un { $percent }%. El modo de bajo consumo reduce la frecuencia de actualización, atenúa ligeramente la pantalla y consulta con menos frecuencia hasta que conecte el cargador.
battery-offer-low-power-action = Activar
battery-sleep-drained = El último reposo gastó más batería de la debida
battery-sleep-drained-body = Consumió { $watts } W durante { $hours } h de reposo, solo el { $percent } % en el estado de reposo más profundo.
battery-sleep-blocked = Impidió el reposo profundo: { $devices }
battery-sleep-woken = Lo despertó: { $devices }
battery-paused = En pausa con batería
battery-paused-body = En pausa hasta que lo conectes a la corriente: { $units }
battery-paused-resume = Reanudar durante esta sesión
battery-paused-keep = Mantener en marcha con batería

## Peripheral Kinds
battery-device-generic = Dispositivo
battery-device-mouse = Ratón
battery-device-touchpad = Panel táctil
battery-device-keyboard = Teclado
battery-device-controller = Mando
battery-device-pen = Lápiz
