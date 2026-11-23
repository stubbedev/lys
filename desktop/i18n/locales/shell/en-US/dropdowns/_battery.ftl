### Battery Dropdown

dropdown-battery-title = Battery

## Hero States
dropdown-battery-on-battery = On Battery
dropdown-battery-charging = Charging
dropdown-battery-plugged-in = Plugged In
dropdown-battery-not-charging = Plugged In, Not Charging
dropdown-battery-critical = Critical

## Time Display
dropdown-battery-duration-hm = { $hours }h { $minutes }m
dropdown-battery-duration-m = { $minutes }m
dropdown-battery-time-remaining = { $duration } remaining
dropdown-battery-time-until-full = { $duration } until full

## Details
dropdown-battery-draw = Draw
dropdown-battery-input = Input
dropdown-battery-input-watts = { $watts } input
dropdown-battery-capacity = Capacity
dropdown-battery-charged = Charged
dropdown-battery-health = Health

## Charge Limit
dropdown-battery-charge-limit = Charge Limit
dropdown-battery-limit-to = Limit to { $threshold }%
dropdown-battery-resumes-at = Resumes charging at { $threshold }%
dropdown-battery-charge-limit-not-supported = Charge limit not supported on this device
dropdown-battery-charge-limit-failed = Couldn't change the charge limit
dropdown-battery-health-cycles = Health, { $cycles ->
    [one] { $cycles } cycle
   *[other] { $cycles } cycles
}
dropdown-battery-hold = Holding at { $limit }% to protect the battery
dropdown-battery-hold-until = Holding at { $limit }% to protect the battery; full by { $time }
dropdown-battery-hold-off = Charging to 100%: no charge limit is set
dropdown-battery-full-requested = Charging to 100% until you unplug
dropdown-battery-full-unplug = Charging to 100% for when you usually unplug, full by { $time }
dropdown-battery-full-event = Charging to 100% for your event at { $time }
dropdown-battery-charge-full = Charge to Full Now
dropdown-battery-charge-full-failed = Couldn't start charging to full

## Power Profile
dropdown-battery-power-profile = Power Profile
dropdown-battery-profile-saver = Saver
dropdown-battery-profile-balanced = Balanced
dropdown-battery-profile-performance = Performance
dropdown-battery-power-profile-not-available = Power profiles daemon must be active

## No Battery
dropdown-battery-no-battery-title = No Battery Detected
dropdown-battery-no-battery-description = Running on AC power

## Peripherals
dropdown-battery-devices = Devices

## Significant Energy
dropdown-battery-energy = Using Significant Energy
dropdown-battery-energy-none = No apps are using significant energy
dropdown-battery-energy-measuring = Measuring…
dropdown-battery-energy-cpu = { $percent }% of a CPU
dropdown-battery-energy-watts = { $watts }, { $percent }% of a CPU
dropdown-battery-energy-show = Show
dropdown-battery-energy-quit = Quit
dropdown-battery-energy-quit-failed = Could not quit { $app }
dropdown-battery-energy-show-failed = Could not show { $app }
dropdown-battery-paused = Paused on Battery
dropdown-battery-paused-now = Paused until you plug in
dropdown-battery-paused-failed = Could not pause
dropdown-battery-paused-resume-failed = Could not resume

## Alerts
battery-alert-low = Battery low
battery-alert-critical = Battery critically low
battery-alert-remaining = { $percent }% remaining
battery-alert-charged = Battery charged
battery-alert-charged-body = Charged to { $percent }%, you can unplug the charger
battery-alert-device-low = { $device } battery low
battery-alert-device-critical = { $device } battery critically low
battery-offer-low-power = Turn on Low Power?
battery-offer-low-power-body = { $percent }% remaining. Low Power lowers the refresh rate, dims the display slightly and polls less until you plug in.
battery-offer-low-power-action = Turn On
battery-sleep-drained = The last sleep used more battery than it should
battery-sleep-drained-body = It drew { $watts } W over { $hours } h asleep, only { $percent }% of it in the deepest sleep state.
battery-sleep-blocked = Kept from deep sleep by { $devices }
battery-sleep-woken = Woken by { $devices }
battery-paused = Paused on battery
battery-paused-body = Paused until you plug in: { $units }
battery-paused-resume = Resume for this session
battery-paused-keep = Keep running on battery

## Peripheral Kinds
battery-device-generic = Device
battery-device-mouse = Mouse
battery-device-touchpad = Touchpad
battery-device-keyboard = Keyboard
battery-device-controller = Controller
battery-device-headset = Headset
battery-device-headphones = Headphones
battery-device-speakers = Speakers
battery-device-phone = Phone
battery-device-tablet = Tablet
battery-device-pen = Pen
battery-device-watch = Watch
battery-device-ups = UPS
battery-device-printer = Printer
battery-device-camera = Camera
battery-device-computer = Computer
battery-device-monitor = Monitor
