### lys-settings: the lys-wm pages

settings-nav-wm-windows = Windows
settings-page-wm-windows = Windows
settings-nav-wm-effects = Shadow & Blur
settings-page-wm-effects = Shadow & Blur
settings-nav-wm-tiling = Tiling & Tabs
settings-page-wm-tiling = Tiling & Tabs
settings-nav-wm-binds = Keyboard Shortcuts
settings-page-wm-binds = Keyboard Shortcuts
settings-nav-wm-outputs = Displays
settings-page-wm-outputs = Displays
settings-nav-wm-session = Rules & Session
settings-page-wm-session = Rules & Session

wm-section-gaps = Gaps
    .description = Space around and between tiled windows
wm-section-borders = Borders
    .description = The line drawn around each window
wm-section-shape = Shape & Transparency
    .description = Corner rounding, and how opaque windows are by focus
wm-section-shadow = Shadow
    .description = The shadow under windows
wm-section-blur = Blur
    .description = What shows through translucent windows
wm-section-overview = Overview
    .description = The view of every workspace at once
wm-section-drag = Drag Preview
    .description = The window that follows the pointer while dragging
wm-section-autotile = Autotile
    .description = Where a new window opens in a tiled layout
wm-section-tabs = Tabs
    .description = The bar over tabbed groups
wm-section-keyboard = Keyboard
    .description = Layout and key repeat
wm-section-pointer = Mouse
    .description = Mice and trackballs
wm-section-touchpad = Touchpad
    .description = Tapping, scrolling and speed
wm-section-trackpoint = Trackpoint
    .description = Pointing sticks in a laptop keyboard
wm-section-binds = Keys & Gestures
    .description = Key and mouse chords and the actions they run
wm-section-outputs = Outputs
    .description = Mode, scale, position and color of each display
wm-section-display = Display
    .description = Settings shared by every display
wm-section-rules = Window Rules
    .description = How matching windows open and look
wm-section-layer-rules = Panels & Overlays
    .description = How the bar, dock, popups and other shell surfaces look
wm-rules-namespace = Surface
wm-rules-blur-alpha-threshold = Blur alpha threshold
wm-layer-lys-shell-bar = Bar
wm-layer-lys-shell-dock = Dock
wm-layer-lys-shell-wallpaper = Wallpaper
wm-layer-lys-shell-launcher = Launcher
wm-layer-lys-shell-notification-popup = Notification popups
wm-layer-lys-shell-osd = Volume and brightness display
wm-layer-lys-shell-power-menu = Power menu
wm-layer-lys-shell-portal-dialog = Portal dialogs
wm-layer-lys-shell-file-chooser = File chooser
wm-layer-lys-shell-print = Print dialog
wm-layer-lys-shell-share-picker = Screen share picker
wm-layer-lys-shell-screenshot-reading = Text recognition
wm-layer-lys-shell-screenshot-thumbnail = Screenshot thumbnail
wm-layer-lys-shell-color-picker = Color picker
wm-layer-lys-shell-region-overlay = Region selection
wm-layer-lys-shell-window-pick = Window picker
wm-section-start = Startup
    .description = What runs with the session, and its environment
wm-section-xwayland = Xwayland
    .description = X11 applications
wm-section-palette = Palette
    .description = Color names the other keys can use

enum-wm-toggle-on = On
enum-wm-toggle-off = Off
wm-status-not-running = Saved. No lys-wm is running here; the next one reads it.
wm-status-unreadable = lys-wm's config cannot be read
wm-status-read-only = lys-wm's config cannot be changed here, so changes are not saved
wm-status-not-applied = lys-wm did not take the change
wm-column-widths-unreadable = The column widths are not ones lys-wm takes; edit them as text
wm-pointers-connected = Connected: { $devices }
wm-pointers-device = { $name } ({ $kind })
wm-pointers-class-pointer = mouse
wm-pointers-class-touchpad = touchpad
wm-pointers-class-trackpoint = trackpoint
wm-pointers-none = No mouse, touchpad or trackpoint is connected.
wm-pointers-not-running = No lys-wm is running here to say which devices are connected, so every section shows.
wm-pointers-outdated = The running lys-wm is older and does not say which devices are connected, so every section shows until you sign in again.
wm-pointers-unreadable = Which devices are connected could not be read, so every section shows
wm-section-devices = Individual Devices
    .description = Settings one device keeps for itself
wm-input-devices = Each pointing device
    .description = A device's own settings win over those of its kind above; one it leaves unset follows them.
wm-input-devices-keyboard = Each keyboard
    .description = A keyboard's own layout wins over the one above; one it leaves unset follows it.
wm-device-absent = Not connected
    .description = This device keeps settings of its own; they apply again when it is connected.
wm-device-forget = Forget this device's settings
wm-devices-none = No device is connected or keeps settings of its own.
wm-devices-not-running = No lys-wm is running here to say which devices are connected; the devices with settings of their own are listed.
wm-devices-outdated = The running lys-wm is older and does not say which devices are connected; the devices with settings of their own are listed until you sign in again.
wm-devices-unreadable = Which devices are connected could not be read, so only the devices with settings of their own are listed
wm-devices-reload = Look for devices again
wm-rules-pick = Pick a window…
wm-rules-picking = Click the window to make a rule for. Escape cancels.
wm-rules-pick-none = The click was not on a window.
wm-rules-pick-gone = The window closed before it could be read.
wm-rules-pick-unnamed = That window has neither an app ID nor a title to match.
wm-rules-pick-not-running = No lys-wm is running here to pick a window from.
wm-rules-pick-failed = The window could not be picked
wm-rules-applies-to = Applies to
wm-rules-then = Then
wm-rules-more-conditions = More conditions
wm-rules-fewer-conditions = Fewer conditions
wm-rules-group-placement = Placement
wm-rules-group-appearance = Appearance
wm-rules-group-behaviour = Behavior
wm-rules-group-performance = Performance
wm-rules-add-effect = Add effect…
wm-rules-no-effects = This rule changes nothing yet: add an effect.
wm-rules-remove-effect = Remove this effect
wm-rules-enabled = Apply this rule
wm-rules-apps-running = Running
wm-rules-border = Border
wm-rules-blur-look = Blur look
wm-preview-windows = Window preview
wm-preview-unreadable = The preview cannot draw { $setting } set to { $value }.
wm-source-file = 
    .description = Set in lys-wm's config file

wm-gaps-inner = Inner gap
    .description = Pixels between tiled windows
    .keywords = spacing, padding, margin
wm-gaps-outer = Outer gap
    .description = One number, or [top, right, bottom, left]
wm-borders-width = Border width
    .description = In pixels; 0 draws none
wm-borders-active = Focused border
    .description = A color, or colors and an angle for a gradient
wm-borders-inactive = Unfocused border
    .description = A color, or colors and an angle for a gradient
wm-decoration-rounding = Corner radius
    .description = In pixels
wm-decoration-active-opacity = Focused opacity
    .description = From 0 to 1
wm-decoration-inactive-opacity = Unfocused opacity
    .description = From 0 to 1
wm-decoration-fullscreen-opacity = Fullscreen opacity
    .description = From 0 to 1
wm-decoration-dim-inactive = Dim unfocused windows
    .description = Darken every window without focus
wm-decoration-dim-strength = Dim strength
    .description = From 0 to 1
wm-decoration-shadow-enabled = Shadows
    .description = Draw a shadow under windows
wm-decoration-shadow-range = Shadow range
    .description = In pixels
wm-decoration-shadow-render-power = Shadow falloff
    .description = From 1 to 4; higher fades faster
wm-decoration-shadow-sharp = Sharp shadows
    .description = Without a soft edge
wm-decoration-shadow-scale = Shadow scale
    .description = Above 0, up to 1
wm-decoration-shadow-offset = Shadow offset
    .description = How far the shadow falls right and down from the window, in pixels; negative moves it left or up
wm-shadow-offset-x = Right
wm-shadow-offset-y = Down
wm-decoration-shadow-color = Shadow color
    .description = #rrggbb[aa] or a palette name; also sets the unfocused color unless that is set
wm-decoration-shadow-color-inactive = Unfocused shadow color
    .description = #rrggbb[aa] or a palette name
wm-decoration-blur-enabled = Blur
    .description = Blur behind every translucent window, not only those that ask
    .keywords = frosted glass, transparency, translucent
wm-decoration-blur-size = Blur size
    .description = In pixels
wm-decoration-blur-passes = Blur passes
    .description = From 1 to 4
wm-decoration-blur-xray = X-ray blur
    .description = Blur only the wallpaper, not the windows below
wm-decoration-blur-tint = Blur tint
    .description = #rrggbb[aa] or a palette name
wm-decoration-blur-saturation = Blur saturation
    .description = From 0 to 4; 1 leaves it unchanged
wm-decoration-blur-brightness = Blur brightness
    .description = From 0 to 4; 1 leaves it unchanged
wm-decoration-blur-contrast = Blur contrast
    .description = From 0 to 4; 1 leaves it unchanged
wm-decoration-blur-grain = Blur grain
    .description = From 0 to 1
wm-animations-enabled = Animations
    .description = Animate windows, workspaces and layers
wm-animations-reduced-motion = Reduced motion
    .description = Fade instead of moving
    .keywords = accessibility, motion sickness
wm-overview-blur = Blur the overview
    .description = Blur what lies behind the overview
wm-overview-dim = Overview dimming
    .description = From 0 to 1
wm-drag-preview-enabled = Drag preview
    .description = Show where a dragged window will land
wm-drag-preview-opacity = Preview opacity
    .description = From 0 to 1
wm-drag-preview-blur = Blur the preview
    .description = Blur behind the preview
wm-drag-preview-snap-distance = Snap distance
    .description = In pixels from an edge
wm-autotile-enabled = Autotile
    .description = Split a window along its longer side
wm-autotile-ephemeral = Ephemeral groups
    .description = A group of one dissolves
wm-autotile-min-trigger = Minimum size to split
    .description = In pixels
wm-autotile-divisor = Split divisor
    .description = Windows narrower than this share of the output stop splitting
wm-tabs-height = Tab bar height
    .description = In pixels
wm-tabs-padding = Tab bar padding
    .description = In pixels
wm-tabs-text-height = Tab text height
    .description = In pixels
wm-tabs-text-padding = Tab text padding
    .description = In pixels
wm-tabs-font = Tab font
    .description = Path to a font file; empty uses the default
wm-tabs-icon-theme = Tab icon theme
    .description = Empty uses the default
wm-input-keyboard-layout = Keyboard layouts
    .description = The layouts you type with, in the order switching goes through them; add one by its name, a variant such as Danish (no dead keys) included
    .keywords = keymap, language, input source
wm-input-sources-misaligned = These layouts and variants do not pair up, more variants than layouts, so both are shown as written; pick a layout or a variant to set them again
wm-input-keyboard-variant = Keyboard variant
    .description = XKB variant
wm-input-keyboard-options = Keyboard options
    .description = XKB options, such as caps:escape
wm-input-keyboard-model = Keyboard model
    .description = XKB model
wm-input-keyboard-rules = Keyboard rules
    .description = XKB rules
wm-input-keyboard-repeat-rate = Repeat rate
    .description = Keys per second
    .keywords = key repeat, typematic
wm-input-keyboard-repeat-delay = Repeat delay
    .description = Milliseconds before a held key repeats
wm-input-pointer-natural-scroll = Natural scrolling
    .description = Content follows the fingers
    .keywords = reverse scrolling, inverted scrolling, mouse wheel
wm-input-pointer-middle-emulation = Middle-click emulation
    .description = Left and right together click the middle button
wm-input-pointer-accel-profile = Acceleration profile
    .description = Flat moves the pointer as far as the device does; adaptive speeds it up with faster motion
wm-input-pointer-accel-speed = Acceleration speed
    .description = From -1 to 1
wm-input-pointer-accel-points = Custom acceleration curve
    .description = The pointer's speed at each step of the device's speed, in place of the profile
wm-input-pointer-scroll-factor = Scroll speed
    .description = Above 0, up to 100; 1 leaves it unchanged
wm-input-trackpoint-natural-scroll = Natural scrolling
    .description = Content follows the fingers
    .keywords = reverse scrolling, inverted scrolling, mouse wheel
wm-input-trackpoint-middle-emulation = Middle-click emulation
    .description = Left and right together click the middle button
wm-input-trackpoint-accel-profile = Acceleration profile
    .description = Flat moves the pointer as far as the device does; adaptive speeds it up with faster motion
wm-input-trackpoint-accel-speed = Acceleration speed
    .description = From -1 to 1
wm-input-trackpoint-accel-points = Custom acceleration curve
    .description = The pointer's speed at each step of the device's speed, in place of the profile
wm-input-trackpoint-scroll-factor = Scroll speed
    .description = Above 0, up to 100; 1 leaves it unchanged
wm-input-trackpoint-scroll-method = Scroll method
    .description = How the trackpoint scrolls: a held button, or none
wm-input-touchpad-natural-scroll = Natural scrolling
    .description = Content follows the fingers
    .keywords = reverse scrolling, inverted scrolling, trackpad
wm-input-touchpad-middle-emulation = Middle-click emulation
    .description = Left and right together click the middle button
wm-input-touchpad-accel-profile = Acceleration profile
    .description = Flat moves the pointer as far as the finger does; adaptive speeds it up with faster motion
wm-input-touchpad-accel-speed = Acceleration speed
    .description = From -1 to 1
wm-input-touchpad-accel-points = Custom acceleration curve
    .description = The pointer's speed at each step of the device's speed, in place of the profile
wm-input-touchpad-scroll-factor = Scroll speed
    .description = Above 0, up to 100; 1 leaves it unchanged
wm-input-touchpad-tap = Tap to click
    .description = A tap clicks, without pressing the touchpad down
    .keywords = tap to click, trackpad
wm-input-touchpad-drag-lock = Drag lock
    .description = A tap-drag survives lifting the finger
wm-input-touchpad-disable-while-typing = Disable while typing
    .description = Ignore the touchpad for a moment after each key press
wm-input-touchpad-click-method = Click method
    .description = How a press chooses the button: by where it lands, or by how many fingers press
wm-input-touchpad-scroll-method = Scroll method
    .description = How the touchpad scrolls: two fingers, the edge, or a held button
wm-binds = Binds
    .description = A list of chord, action and flags; a config without any has none
wm-submaps = Submaps
    .description = Named sets of binds
wm-gestures = Gestures
    .description = Touchpad swipes, pinches and holds
wm-outputs = Output rules
    .description = Each output takes the first rule that matches it
wm-color-management = Color management
    .description = Offer clients the color-management protocol
wm-tearing = Tearing
    .description = Let windows that ask for it tear in fullscreen
wm-gpu = GPU
    .description = A card or render node to use; empty picks one
wm-lid-action = On lid close
    .description = What closing the lid does on battery with no external display connected: suspend, hibernate, hybrid sleep, suspend then hibernate, or nothing
wm-rules = Rules
    .description = A list of match and effect
wm-layer-rules = Rules
    .description = Each rule changes how the shell surfaces it applies to look
wm-autostart = Autostart
    .description = Commands run once the session is up
wm-env = Environment
    .description = Variables for lys-wm and the apps it starts; a change reaches apps started after it, not ones already running
wm-env-name-placeholder = Variable, such as GDK_BACKEND
wm-env-name-invalid = A variable name holds only letters, digits and _, and does not start with a digit; it is not saved until it does
wm-env-moz-wayland = Firefox runs on Wayland
wm-env-qt-wayland = Qt apps run on Wayland
wm-env-gdk-wayland = GTK apps run on Wayland
wm-env-sdl-wayland = SDL games run on Wayland
wm-env-electron-wayland = Electron apps run on Wayland where they can
wm-env-java-tiling = Java apps draw their windows correctly
wm-xwayland-enabled = Xwayland
    .description = Run X11 applications
wm-xwayland-zero-scaling = Zero scaling
    .description = X11 apps draw at the screen’s real pixels instead of being scaled up blurry; they look small on a scaled screen unless they scale themselves
wm-palette = Palette
    .description = Named colors other keys can use

## Structured keys
wm-section-gestures = Gestures
    .description = Touchpad gestures and what they drive
wm-gestures-swipe = Swipes
    .description = Touchpad swipes and what each drives or runs
wm-gestures-pinch = Pinches
    .description = Touchpad pinches and what each drives or runs
wm-gestures-hold = Holds
    .description = Fingers held on the touchpad and the action each runs
wm-gestures-distance = Gesture distance
    .description = Touchpad units for one whole swipe, about 12 to the millimetre
wm-card-number = Item { $number }
wm-side-top = Top
wm-side-right = Right
wm-side-bottom = Bottom
wm-side-left = Left
wm-border-add-color = Add a color
wm-border-angle = Angle
wm-binds-action = Action
wm-binds-chord = Chord
wm-binds-flags = Flags
wm-binds-cooldown = Cooldown (ms)
wm-binds-description = Description
wm-binds-long-press = Long press (ms)
wm-gestures-action = Action
wm-gestures-direction = Direction
wm-gestures-drive = Drives
wm-gestures-fingers = Fingers
wm-gestures-modifiers = Modifiers
wm-gestures-scope = Overview scope
wm-outputs-bitdepth = Bit depth
    .description = 8 or 10 bits per channel; 10 smooths gradients under night light
wm-outputs-disabled = Disabled
wm-outputs-hdr = HDR
    .description = Drive the display in HDR always, or only for fullscreen HDR content
wm-outputs-hdr-peak = HDR peak brightness
    .description = With HDR: the display's peak brightness in cd/m², if its EDID has it wrong
wm-outputs-icc = ICC profile
    .description = A matrix-shaper ICC profile for an SDR display
wm-outputs-hot-corners = Hot corners
    .description = Off turns every hot corner off on this display; On changes some of them here
wm-outputs-hot-corners-top-left = Top left corner
wm-outputs-hot-corners-top-left-action = Action
wm-outputs-hot-corners-top-left-modifiers = Modifiers
wm-outputs-hot-corners-top-right = Top right corner
wm-outputs-hot-corners-top-right-action = Action
wm-outputs-hot-corners-top-right-modifiers = Modifiers
wm-outputs-hot-corners-bottom-left = Bottom left corner
wm-outputs-hot-corners-bottom-left-action = Action
wm-outputs-hot-corners-bottom-left-modifiers = Modifiers
wm-outputs-hot-corners-bottom-right = Bottom right corner
wm-outputs-hot-corners-bottom-right-action = Action
wm-outputs-hot-corners-bottom-right-modifiers = Modifiers
wm-outputs-match = Match
wm-outputs-mirror = Mirror
    .description = Another display whose image this one shows instead of its own desktop
wm-outputs-mode = Mode
wm-outputs-position = Position
wm-outputs-reserved = Reserved space
    .description = Logical pixels kept free of tiled and maximized windows: top, right, bottom, left
wm-outputs-scale = Scale
wm-outputs-sdr-brightness = SDR brightness
    .description = With HDR: the cd/m² SDR white shows at
wm-outputs-shader = Shader
    .description = A fragment shader for the display's final image
wm-outputs-transform = Rotation
    .description = How the panel is turned or flipped
wm-outputs-vrr = Variable refresh rate
    .description = Adaptive sync: off, always on, or only for a fullscreen game or video
wm-rules-animation = Animation
wm-rules-animation-in = Opening animation
wm-rules-animation-out = Closing animation
wm-rules-app-id = App ID
wm-rules-block-out-from = Hide from captures
wm-rules-blur = Blur
wm-rules-blur-look-brightness = Blur brightness
wm-rules-blur-look-contrast = Blur contrast
wm-rules-blur-look-grain = Blur grain
wm-rules-blur-look-refraction-amount = Refraction amount
wm-rules-blur-look-refraction-bezel = Refraction bezel
wm-rules-blur-look-saturation = Blur saturation
wm-rules-blur-look-specular-strength = Specular strength
wm-rules-blur-look-specular-width = Specular width
wm-rules-blur-look-tint = Blur tint
wm-rules-border-active = Active border
wm-rules-border-inactive = Inactive border
wm-rules-border-width = Border width
wm-rules-dim-around = Dim around
wm-rules-dim-inactive = Dim when inactive
wm-rules-float = Float
wm-rules-mica = Mica
wm-rules-opacity = Opacity
wm-rules-size = Size
wm-rules-tag = Tag
wm-rules-tearing = Tearing
wm-rules-touch-gestures = Touch gestures
wm-rules-tile = Tile
wm-rules-vrr = Variable refresh rate
wm-rules-title = Title
wm-rules-exclude-app-id = Except app ID
wm-rules-exclude-is-captured = Except captured
wm-rules-exclude-is-floating = Except floating
wm-rules-exclude-is-focused = Except focused
wm-rules-exclude-is-fullscreen = Except fullscreen
wm-rules-exclude-is-urgent = Except urgent
wm-rules-exclude-initial-app-id = Except initial app ID
wm-rules-exclude-initial-title = Except initial title
wm-rules-exclude-tag = Except tag
wm-rules-exclude-title = Except title
wm-rules-exclude-xwayland = Except Xwayland
wm-rules-focus = Focus
wm-rules-fullscreen = Fullscreen
wm-rules-group = Group
wm-rules-idle-inhibit = Inhibit idle
wm-rules-initial-app-id = Initial app ID
wm-rules-initial-title = Initial title
wm-rules-is-captured = Is captured
wm-rules-is-floating = Is floating
wm-rules-is-focused = Is focused
wm-rules-is-fullscreen = Is fullscreen
wm-rules-is-urgent = Is urgent
wm-rules-keep-aspect = Keep aspect ratio
wm-rules-max-size = Maximum size
wm-rules-min-size = Minimum size
wm-rules-maximized = Maximized
wm-rules-output = Output
wm-rules-pin = Pin
wm-rules-position = Position
wm-rules-rounding = Corner radius
wm-rules-scroll-factor = Scroll factor
wm-rules-shadow = Shadow
wm-rules-suppress = Ignored requests
wm-rules-workspace = Workspace
wm-rules-xwayland = Xwayland
wm-section-focus = Focus
    .description = What a window that asks for focus gets
wm-focus-on-activate = On activation
    .description = Focus a window that asks right after you used the app that asked, else mark it urgent; or always mark it urgent
wm-rules-on-activate = On activation
    .description = What this window gets when it asks for focus: focus, urgent, or nothing at all
wm-section-app-shortcuts = App Shortcuts
    .description = Shortcuts apps registered through the portal, each pressed through a global bind
wm-app-shortcuts = Registered shortcuts
    .description = Record a chord to bind it, Backspace to unbind it
wm-app-shortcuts-none = No app has registered a shortcut yet
wm-app-shortcuts-hotkey = The app holds this key itself, set in its portal dialog
wm-rules-content = Content
    .description = What the window shows (photo, video or game), over what it says itself
wm-app-shortcuts-reload = Look again
wm-section-window-behavior = Window Behavior
    .description = How new, floating and fullscreen windows behave
wm-section-titlebars = Title Bars
    .description = Bars with buttons above floating windows
wm-section-mica = Mica
    .description = A heavy blur of the wallpaper behind translucent windows
wm-section-hot-corners = Hot Corners
    .description = What pushing the pointer into a screen corner does
wm-section-layout = Layout
    .description = How a new workspace arranges its windows
wm-section-cursor = Pointer
    .description = When the pointer hides and how to find it
wm-focus-follows-mouse = Focus follows pointer
    .description = Hovering a window focuses it; loose waits until the pointer enters another window
wm-focus-raise-on-hover = Raise on hover
    .description = Hover focus also brings a floating window to the front
wm-focus-warp-pointer = Move pointer with focus
    .description = Center the pointer on a window a shortcut focuses, or only when a shortcut switches workspace
wm-focus-on-close = Focus after closing
    .description = Which window gets focus when the focused one closes
wm-fullscreen-new-window = New window over fullscreen
    .description = Whether a window opening over a fullscreen or maximized one stays behind, takes over, or ends it
wm-hold-opening = Wait for neighbors
    .description = A new window waits up to 100 ms for resized neighbors to redraw before it shows
wm-floating-step = Floating move step
    .description = Pixels a shortcut moves a floating window
wm-section-swallow = Terminal Swallowing
    .description = A terminal hides behind the window it opens until that window closes
wm-swallow-terminals = Terminals that swallow
    .description = A window started from one of these terminals takes its place until it closes
wm-swallow-except-title = Titles that never swallow
    .description = A regular expression on the new window's title; matching windows open beside the terminal
wm-swallow-terminals-none = No terminal swallows the windows it starts
wm-swallow-terminals-add = Add a terminal
wm-swallow-terminals-written = A pattern written by hand, which a list of terminals cannot express
wm-swallow-terminals-replace = Replace with a list
wm-titlebars-floating = Show title bars
    .description = Bars with buttons above floating windows
wm-titlebars-height = Title bar height
    .description = Pixels, taken from the floating window
wm-titlebars-buttons = Buttons
    .description = Which buttons the bar shows, in order
wm-titlebars-side = Button side
    .description = Which end of the bar holds the buttons
wm-titlebars-double-click = Double-click
    .description = What double-clicking the bar does
wm-titlebars-active-background = Title bar background, focused
    .description = The title bar's color when focused
wm-titlebars-active-text = Title bar text, focused
    .description = The title bar's text color when focused
wm-titlebars-inactive-background = Title bar background, unfocused
    .description = The title bar's color when unfocused
wm-titlebars-inactive-text = Title bar text, unfocused
    .description = The title bar's text color when unfocused
wm-titlebars-urgent-background = Title bar background, urgent
    .description = The title bar's color when urgent
wm-titlebars-urgent-text = Title bar text, urgent
    .description = The title bar's text color when urgent
wm-tabs-active-background = Tab background, focused
    .description = The tab's color when focused
wm-tabs-active-text = Tab text, focused
    .description = The tab's text color when focused
wm-tabs-inactive-background = Tab background, unfocused
    .description = The tab's color when unfocused
wm-tabs-inactive-text = Tab text, unfocused
    .description = The tab's text color when unfocused
wm-tabs-urgent-background = Tab background, urgent
    .description = The tab's color when urgent
wm-tabs-urgent-text = Tab text, urgent
    .description = The tab's text color when urgent
wm-titlebars-lights-close = Close button color
    .description = Color of the close button's dot
wm-titlebars-lights-minimize = Minimize button color
    .description = Color of the minimize button's dot
wm-titlebars-lights-maximize = Maximize button color
    .description = Color of the maximize button's dot
wm-decoration-blur-live-tiled = Live blur for tiled windows
    .description = Blur what is behind each tiled window instead of the shared background blur
wm-decoration-blur-mica-enabled = Mica
    .description = Draw translucent windows over a heavy blur of the wallpaper
wm-decoration-blur-mica-size = Mica blur size
    .description = Distance between blur taps, in pixels
wm-decoration-blur-mica-passes = Mica blur passes
    .description = Each pass doubles the blur's reach
wm-decoration-blur-mica-tint = Mica tint
    .description = Color laid over the mica backdrop
wm-drag-preview-snap-windows = Snap to windows
    .description = Floating windows also snap beside or in line with other floating windows
wm-drag-preview-drag-switch-width = Edge switch zone
    .description = Pixels at a screen edge with no neighbor where holding a drag switches workspace
wm-drag-preview-drag-switch-frames = Edge switch delay
    .description = Frames a drag must be held at the edge before it switches
wm-hot-corners-top-left = Top left corner
    .description = Action run when the pointer is pushed into this corner
wm-hot-corners-top-right = Top right corner
    .description = Action run when the pointer is pushed into this corner
wm-hot-corners-bottom-left = Bottom left corner
    .description = Action run when the pointer is pushed into this corner
wm-hot-corners-bottom-right = Bottom right corner
    .description = Action run when the pointer is pushed into this corner
wm-hot-corners-corner-action = Action
    .description = What the corner does
wm-hot-corners-corner-modifiers = Hold keys
    .description = Keys that must be held for the corner to fire
wm-hot-corners-pressure = Corner push
    .description = How far the pointer must push into a corner before it fires
wm-hot-corners-fullscreen = Corners in fullscreen
    .description = Corners also fire while a fullscreen window has focus
wm-showdesktop-peek = Show desktop peek
    .description = Percent of each window left visible when the desktop is shown
wm-switcher-open-delay = Window switcher delay
    .description = Milliseconds the keys must be held before the switcher strip shows
wm-layout = New workspace layout
    .description = Splits and tab groups, or a scrolling strip of columns
wm-scroll-center = Center focused column
    .description = When the strip scrolls the focused column to the middle
wm-input-keyboard-numlock = Num Lock on
    .description = Turn Num Lock on at startup and after every reload
wm-input-keyboard-sticky-keys = Sticky keys
    .description = A modifier tapped alone applies to the next key; tapped twice it locks
wm-input-keyboard-slow-keys = Slow keys
    .description = Milliseconds a key must be held to count; 0 is off
wm-input-pointer-tap = Tap to click
    .description = A tap clicks, without pressing down
wm-input-pointer-drag-lock = Drag lock
    .description = Keep a tap-and-drag going after the finger lifts
wm-input-pointer-disable-while-typing = Off while typing
    .description = Ignore the device while keys are typed
wm-input-pointer-disabled-on-external-mouse = Off with a mouse plugged in
    .description = Turn this device off while another pointer is connected
wm-input-pointer-left-handed = Left-handed
    .description = Swap the primary and secondary buttons
wm-input-pointer-click-method = Click method
    .description = Where on the device a click is read: button areas or finger count
wm-input-pointer-scroll-method = Scroll method
    .description = Two fingers, the edge, or holding a button
wm-input-pointer-scroll-button-lock = Scroll button toggles
    .description = A click on the scroll button turns scrolling on and off
wm-input-pointer-tap-button-map = Tap buttons
    .description = Which buttons one-, two- and three-finger taps press
wm-input-touchpad-disabled-on-external-mouse = Off with a mouse plugged in
    .description = Turn this device off while another pointer is connected
wm-input-touchpad-left-handed = Left-handed
    .description = Swap the primary and secondary buttons
wm-input-touchpad-scroll-button-lock = Scroll button toggles
    .description = A click on the scroll button turns scrolling on and off
wm-input-touchpad-tap-button-map = Tap buttons
    .description = Which buttons one-, two- and three-finger taps press
wm-input-trackpoint-tap = Tap to click
    .description = A tap clicks, without pressing down
wm-input-trackpoint-drag-lock = Drag lock
    .description = Keep a tap-and-drag going after the finger lifts
wm-input-trackpoint-disable-while-typing = Off while typing
    .description = Ignore the device while keys are typed
wm-input-trackpoint-disabled-on-external-mouse = Off with a mouse plugged in
    .description = Turn this device off while another pointer is connected
wm-input-trackpoint-left-handed = Left-handed
    .description = Swap the primary and secondary buttons
wm-input-trackpoint-click-method = Click method
    .description = Where on the device a click is read: button areas or finger count
wm-input-trackpoint-scroll-button-lock = Scroll button toggles
    .description = A click on the scroll button turns scrolling on and off
wm-input-trackpoint-tap-button-map = Tap buttons
    .description = Which buttons one-, two- and three-finger taps press
wm-gestures-edge-width = Touch edge zone
    .description = Pixels from a screen edge where an edge swipe starts
wm-cursor-hide-while-typing = Hide while typing
    .description = Hide the pointer while you type into a window
wm-cursor-hide-after = Hide when idle
    .description = Milliseconds without pointer movement before it hides; 0 never
wm-cursor-shake-to-find = Shake to find
    .description = Shaking the mouse briefly enlarges the pointer
wm-hidden-frame-rate = Hidden window frame rate
    .description = Frames per second for windows that are fully covered or off screen
wm-power-saver-frame-rate = Power saver frame rate
    .description = Highest frame rate while the power saver profile is on; 0 is off
wm-client-hotkeys = App global shortcuts
    .description = Let apps bind global shortcuts that the focused window does not see
wm-side-all = All sides
wm-side-same = Same on all sides
wm-side-window = Window
enum-wm-accel-profile-adaptive = Adaptive
enum-wm-accel-profile-flat = Flat
enum-wm-center-always = Always
enum-wm-center-never = Never
enum-wm-center-on-overflow = When it overflows
enum-wm-click-method-button_areas = Button areas
enum-wm-click-method-clickfinger = Number of fingers
enum-wm-click-method-none = None
enum-wm-double-click-float = Float
enum-wm-double-click-maximize = Maximize
enum-wm-follows-mouse-always = Strictly under the pointer
enum-wm-follows-mouse-loose = When the pointer enters another window
enum-wm-follows-mouse-off = Click to focus
enum-wm-layout-scroll = Scrolling columns
enum-wm-layout-tree = Tiling tree
enum-wm-lid-action-auto = Suspend, then hibernate if possible
enum-wm-lid-action-hibernate = Hibernate
enum-wm-lid-action-hybrid-sleep = Hybrid sleep
enum-wm-lid-action-ignore = Do nothing
enum-wm-lid-action-suspend = Suspend
enum-wm-lid-action-suspend-then-hibernate = Suspend, then hibernate
enum-wm-new-window-behind = Opens behind
enum-wm-new-window-exit = Leaves full screen
enum-wm-new-window-take_over = Takes over full screen
enum-wm-on-activate-focus = Focus it
enum-wm-on-activate-urgent = Mark it urgent
enum-wm-on-close-next = The next window
enum-wm-on-close-under_pointer = The window under the pointer
enum-wm-scroll-method-edge = Edge
enum-wm-scroll-method-none = None
enum-wm-scroll-method-on_button_down = While a button is held
enum-wm-scroll-method-two_finger = Two fingers
enum-wm-side-left = Left
enum-wm-side-right = Right
enum-wm-tap-button-map-lmr = Left, middle, right
enum-wm-tap-button-map-lrm = Left, right, middle
enum-wm-warp-pointer-off = Never
enum-wm-warp-pointer-on_keyboard_focus = When the keyboard moves focus
enum-wm-warp-pointer-on_workspace_switch = When switching workspace
settings-nav-wm-keyboard = Keyboard
settings-page-wm-keyboard = Keyboard
settings-nav-wm-pointer = Mouse & Touchpad
settings-page-wm-pointer = Mouse & Touchpad
wm-section-typing = Typing
    .description = How fast held keys repeat, and Num Lock at start
wm-section-keyboard-accessibility = Accessibility
    .description = Keys that stick or wait, for typing one key at a time
wm-section-keyboard-model = Keyboard model
    .description = The keyboard model and rules the layouts are built for
wm-palette-new-name = New colour name
wm-palette-add = Add colour
wm-palette-reset = Back to lys-wm’s colour
wm-palette-taken = There is already a colour called { $name }
wm-palette-add-failed = The colour { $name } was not added
wm-palette-reset-failed = The colour { $name } was not reset

## What a bind does, by lys-wm action: $first is the first argument, or default without any; $args all of them; $rest all after the first
wm-action-focus = { $first ->
        [left] Focus the window on the left
        [right] Focus the window on the right
        [up] Focus the window above
        [down] Focus the window below
       *[other] Focus { $args }
    }
wm-action-move = { $first ->
        [left] Move window left
        [right] Move window right
        [up] Move window up
        [down] Move window down
       *[other] Move window { $args }
    }
wm-action-workspace = { $first ->
        [previous] Switch to the previous workspace
        [empty] Switch to an empty workspace
       *[other] Switch to workspace { $args }
    }
wm-action-movetoworkspace = { $first ->
        [previous] Move window to the previous workspace
        [empty] Move window to an empty workspace
       *[other] Move window to workspace { $args }
    }
wm-action-focusoutput = { $first ->
        [left] Focus the display on the left
        [right] Focus the display on the right
        [up] Focus the display above
        [down] Focus the display below
       *[other] Focus display { $args }
    }
wm-action-movetooutput = { $first ->
        [left] Move window to the display on the left
        [right] Move window to the display on the right
        [up] Move window to the display above
        [down] Move window to the display below
       *[other] Move window to display { $args }
    }
wm-action-moveworkspacetooutput = { $first ->
        [left] Move workspace to the display on the left
        [right] Move workspace to the display on the right
        [up] Move workspace to the display above
        [down] Move workspace to the display below
       *[other] Move workspace to display { $args }
    }
wm-action-swapworkspaces = Swap the workspaces of displays { $args }
wm-action-scratchpad = { $first ->
        [hide] Hide the scratchpad
       *[other] Toggle scratchpad { $rest }
    }
wm-action-movetoscratchpad = Move window to scratchpad { $args }
wm-action-kill = Close window
wm-action-focuswindow = Focus window { $args }
wm-action-minimize = { $first ->
        [default] Minimize window
       *[other] Minimize window { $args }
    }
wm-action-restore = { $first ->
        [default] Restore the last minimized window
       *[other] Restore window { $args }
    }
wm-action-overview = { $first ->
        [show] Show the overview
        [hide] Hide the overview
       *[other] Toggle the overview
    }
wm-action-closewindow = Close window { $args }
wm-action-dpms = { $first ->
        [on] Turn displays on
        [off] Turn displays off
       *[other] Turn displays on or off
    }
wm-action-reload = Reload the configuration
wm-action-dismissconfignotice = Dismiss the configuration notice
wm-action-exit = End the session
wm-action-nextlayout = Switch to the next keyboard layout
wm-action-stickykeys = { $first ->
        [on] Turn sticky keys on
        [off] Turn sticky keys off
       *[other] Sticky keys { $args }
    }
wm-action-slowkeys = Set slow keys to { $args } ms
wm-action-global = App shortcut { $args }
wm-action-exec = Run { $args }
wm-action-launch = Launch { $app }
wm-action-makegroup = { $first ->
        [h] Group window side by side
        [v] Group window stacked
        [tab] Group window as tabs
       *[other] Group window { $args }
    }
wm-action-setlayout = { $first ->
        [tree] Use the tiling layout
        [scroll] Use the scrolling layout
       *[other] Set layout { $args }
    }
wm-action-columnwidth = Set column width { $args }
wm-action-consume = { $first ->
        [left] Move window into the column on the left
        [right] Move window into the column on the right
       *[other] Move window into column { $args }
    }
wm-action-expel = Move window into a column of its own
wm-action-center = Center the focused column
wm-action-columndisplay = { $first ->
        [stack] Stack the windows of the column
        [tab] Show the column as tabs
       *[other] Show the column as { $args }
    }
wm-action-changegroup = { $first ->
        [h] Arrange the group side by side
        [v] Arrange the group stacked
        [tab] Arrange the group as tabs
        [toggletab] Toggle tabs in the group
        [opposite] Turn the group the other way
       *[other] Change group { $args }
    }
wm-action-focustab = { $first ->
        [prev] Focus the previous tab
        [next] Focus the next tab
       *[other] Focus tab { $args }
    }
wm-action-focusparent = Focus the surrounding group
wm-action-focuschild = Focus back into the group
wm-action-swap = { $first ->
        [left] Swap window with the one on the left
        [right] Swap window with the one on the right
        [up] Swap window with the one above
        [down] Swap window with the one below
       *[other] Swap window { $args }
    }
wm-action-moveout = Move window out of its group
wm-action-equalize = { $first ->
        [all] Make every split even
       *[other] Make the group even
    }
wm-action-locktab = { $first ->
        [on] Lock the tab group
        [off] Unlock the tab group
       *[other] Lock or unlock the tab group
    }
wm-action-submap = { $first ->
        [reset] Leave the mode
       *[other] Enter mode { $args }
    }
wm-action-togglefloating = Float or tile window
wm-action-place = { $first ->
        [left] Place window on the left half
        [right] Place window on the right half
        [top] Place window on the top half
        [bottom] Place window on the bottom half
        [topleft] Place window in the top left quarter
        [topright] Place window in the top right quarter
        [bottomleft] Place window in the bottom left quarter
        [bottomright] Place window in the bottom right quarter
        [fill] Let window fill the screen
        [center] Center window
        [restore] Put window back where it was
       *[other] Place window { $args }
    }
wm-action-focusfloating = { $first ->
        [on] Focus the floating windows
        [off] Focus the tiled windows
       *[other] Switch focus between floating and tiled windows
    }
wm-action-fullscreen = { $first ->
        [on] Make window fullscreen
        [off] Leave fullscreen
       *[other] Toggle fullscreen
    }
wm-action-setprop = Set window property { $args }
wm-action-showdesktop = { $first ->
        [on] Show the desktop
        [off] Bring the windows back
       *[other] Show or hide the desktop
    }
wm-action-swallow = { $first ->
        [on] Hide the terminal in the window
        [off] Show the hidden terminal
       *[other] Show or hide the swallowed terminal
    }
wm-action-pin = { $first ->
        [on] Pin window
        [off] Unpin window
       *[other] Pin or unpin window
    }
wm-action-maximize = { $first ->
        [on] Maximize window
        [off] Unmaximize window
       *[other] Toggle maximize
    }
wm-action-resizeactive = Resize window by { $args }
wm-action-movewindow = Drag window
wm-action-resizewindow = Resize window by dragging
wm-action-zoom = { $first ->
        [in] Zoom in
        [out] Zoom out
        [reset] Reset zoom
       *[other] Set zoom to { $rest }
    }
wm-action-debug = Debug { $args }
wm-action-casttarget = { $first ->
        [window] Share a window
        [output] Share a display
        [clear] Stop sharing
       *[other] Share { $args }
    }
wm-action-focuslast = Focus the previous window
wm-action-focusurgent = Focus the window asking for attention
wm-action-cycle = { $first ->
        [next] Cycle to the next window
        [prev] Cycle to the previous window
       *[other] Cycle windows { $args }
    }
wm-action-switcher = { $first ->
        [next] Switch to the next window
        [prev] Switch to the previous window
       *[other] Switch windows { $args }
    }
wm-binds-unknown-action = Unknown action
    .description = lys-wm has no action by this name; the bind is shown as written
wm-binds-category-windows = Windows
wm-binds-category-workspaces = Workspaces
wm-binds-category-layout = Layout
wm-binds-category-apps = Apps
wm-binds-category-screenshots = Screenshots
wm-binds-category-media = Media
wm-binds-category-system = System
wm-binds-category-custom = Custom
wm-binds-search = Search shortcuts, actions or keys

## The arguments of a lys-wm action, by the form lys-wm describes
wm-action-pick = Choose an action
wm-action-args-outside = Not a form of this action
    .description = lys-wm describes this action’s arguments otherwise; edit them as text
enum-wm-arg-default = Default
enum-wm-arg-all = All
enum-wm-arg-app = Focused app
enum-wm-arg-app_id = App ID
enum-wm-arg-app_id-colon-id = App and shortcut
enum-wm-arg-bottom = Bottom half
enum-wm-arg-bottomleft = Bottom left
enum-wm-arg-bottomright = Bottom right
enum-wm-arg-center = Center
enum-wm-arg-clear = Nothing
enum-wm-arg-command = Command
enum-wm-arg-down = Down
enum-wm-arg-dx = Width change (px)
enum-wm-arg-dy = Height change (px)
enum-wm-arg-e-minus-n = Open workspace before, any display
enum-wm-arg-e-plus-n = Open workspace after, any display
enum-wm-arg-empty = First empty
enum-wm-arg-ephemeral = Dissolve when one window is left
enum-wm-arg-factor = Factor
enum-wm-arg-fake = Only inside its tile
enum-wm-arg-fill = Fill
enum-wm-arg-follow = Follow the window
enum-wm-arg-h = Side by side
enum-wm-arg-hide = Hide
enum-wm-arg-id = Window ID
enum-wm-arg-in = In
enum-wm-arg-key = Property
enum-wm-arg-left = Left
enum-wm-arg-m-minus-n = Open workspace before, this display
enum-wm-arg-m-plus-n = Open workspace after, this display
enum-wm-arg-minus-n-percent = Narrower by percent
enum-wm-arg-minus-n-px = Narrower by pixels
enum-wm-arg-ms = Milliseconds
enum-wm-arg-n = Number
enum-wm-arg-n-percent = Percent of the display
enum-wm-arg-n-px = Pixels
enum-wm-arg-name = Name
enum-wm-arg-next = Next
enum-wm-arg-off = Off
enum-wm-arg-on = On
enum-wm-arg-opposite = The other way
enum-wm-arg-out = Out
enum-wm-arg-output = Display
enum-wm-arg-output-name = Display
enum-wm-arg-plus-n-percent = Wider by percent
enum-wm-arg-plus-n-px = Wider by pixels
enum-wm-arg-preset-next = Next preset
enum-wm-arg-preset-prev = Previous preset
enum-wm-arg-prev = Previous
enum-wm-arg-previous = Previous
enum-wm-arg-reset = Reset
enum-wm-arg-restore = Where it was
enum-wm-arg-right = Right
enum-wm-arg-scroll = Scrolling
enum-wm-arg-set = Set
enum-wm-arg-show = Show
enum-wm-arg-stack = Stacked
enum-wm-arg-tab = Tabs
enum-wm-arg-toggle = Toggle
enum-wm-arg-toggle-name = Toggle
enum-wm-arg-toggletab = Toggle tabs
enum-wm-arg-top = Top half
enum-wm-arg-topleft = Top left
enum-wm-arg-topright = Top right
enum-wm-arg-tree = Tiling
enum-wm-arg-up = Up
enum-wm-arg-v = Stacked
enum-wm-arg-value = Value
enum-wm-arg-window = Window
enum-wm-arg-window-id = Window
enum-wm-arg-workspace = Workspace
enum-wm-arg-wrap = Wrap around
wm-binds-clash-mode = { $label } in mode { $mode }
wm-binds-reset = Back to lys-wm’s shortcut
wm-mode = Mode { $mode }
wm-mode-enter = Shortcut entering mode { $mode }
wm-mode-rename = Rename the mode
wm-mode-rename-confirm = Rename
wm-mode-delete-button = Delete the mode
wm-mode-delete-title = Delete mode { $mode }?
wm-mode-delete-description = Its shortcuts, and the shortcuts that enter it, are removed too.
wm-mode-delete-confirm = Delete
wm-mode-add = Add a mode
wm-mode-add-confirm = Add
wm-mode-name = Name
wm-mode-name-placeholder = Name, such as resize
wm-mode-chord = Shortcut entering the mode
wm-mode-name-missing = A mode needs a name
wm-mode-name-reset = { $name } is how a shortcut leaves a mode, so it cannot name one
wm-mode-name-taken = A mode named { $name } exists already
wm-mode-chord-missing = Record the shortcut that enters the mode
wm-mode-refused = lys-wm does not take this change
wm-mode-not-saved = The change was not saved
wm-mode-delete-failed = The mode could not be deleted
wm-mode-unreachable = No shortcut enters this mode; record one beside its name
wm-section-refraction = Refraction
    .description = Blurred glass that bends what is behind it near its edges
wm-section-specular = Rim sheen
    .description = A highlight along the edge of blurred glass, on the side facing the pointer
wm-decoration-blur-refraction = Refraction
    .description = Bend the backdrop near the edges of blurred windows
wm-decoration-blur-refraction-bezel = Bending edge
    .description = How far in from the edge the backdrop bends, in pixels
wm-decoration-blur-refraction-amount = Refraction strength
    .description = How far past the edge the glass looks, in pixels
wm-decoration-blur-specular = Rim sheen
    .description = Light the edge of blurred windows
wm-decoration-blur-specular-strength = Sheen brightness
    .description = How brightly the rim shines
wm-decoration-blur-specular-width = Sheen width
    .description = In pixels
wm-scroll-presets = Column widths
    .description = The widths the column width shortcut steps through in the scrolling layout, in this order
    .keywords = presets, columnwidth, scrolling
wm-scroll-presets-share = Share of the screen
wm-scroll-presets-pixels = Pixels
wm-scroll-presets-add = Add width
wm-scroll-presets-add-width = Add a column { $width } wide
wm-scroll-presets-remove = Remove this width
wm-section-touch-gestures = Touchscreen gestures
    .description = Swipes on a touchscreen and what they do
wm-gestures-touch = Touch swipes
    .description = Swipes in from an edge, or with three or more fingers anywhere on the screen
wm-gestures-touch-add = Add touch swipe
wm-gestures-touch-start = Starts
wm-gestures-touch-does = Does
wm-gestures-touch-fingers-anywhere = Fingers anywhere
wm-gestures-touch-fingers = Fingers
wm-gestures-touch-direction = Direction
wm-gestures-touch-run = Run an action
wm-gestures-touch-follow = Follow the fingers
wm-gestures-touch-edge-title = { $edge ->
        [top] Swipe in from the top edge
        [bottom] Swipe in from the bottom edge
        [left] Swipe in from the left edge
        [right] Swipe in from the right edge
       *[other] Swipe in from an edge
    }
wm-gestures-touch-fingers-title = { $direction ->
        [left] { $fingers }-finger swipe left
        [right] { $fingers }-finger swipe right
        [up] { $fingers }-finger swipe up
        [down] { $fingers }-finger swipe down
        [horizontal] { $fingers }-finger swipe sideways
        [vertical] { $fingers }-finger swipe up or down
        [any] { $fingers }-finger swipe any way
       *[other] { $fingers }-finger swipe
    }
enum-wm-edge-top = Top edge
enum-wm-edge-bottom = Bottom edge
enum-wm-edge-left = Left edge
enum-wm-edge-right = Right edge
enum-wm-direction-left = Left
enum-wm-direction-right = Right
enum-wm-direction-up = Up
enum-wm-direction-down = Down
enum-wm-direction-horizontal = Sideways
enum-wm-direction-vertical = Up or down
enum-wm-direction-any = Any way
enum-wm-drive-workspaces = Workspaces
enum-wm-drive-overview = Overview
enum-wm-drive-view = Scroll columns
enum-wm-drive-move = Move window
enum-wm-drive-resize = Resize window
wm-input-pointer-scroll-button = Scroll button
    .description = The button held to scroll by moving the device
wm-input-touchpad-scroll-button = Scroll button
    .description = The button held to scroll by moving the device
wm-input-trackpoint-scroll-button = Scroll button
    .description = The button held to scroll by moving the device
wm-button-code = Button code
    .description = Another button, by its evdev code (BTN_SIDE is 275)
enum-wm-scroll-button-back = Back
enum-wm-scroll-button-extra = Extra
enum-wm-scroll-button-forward = Forward
enum-wm-scroll-button-left = Left
enum-wm-scroll-button-middle = Middle
enum-wm-scroll-button-right = Right
enum-wm-scroll-button-side = Side
enum-wm-scroll-button-task = Task
wm-preview-tabs = Tab bar preview
wm-preview-tab-active = Focused
wm-preview-tab-inactive = Other window
wm-preview-tab-urgent = Wants attention
wm-tabs-font-built-in = lys-wm's own font
wm-tabs-font-unresolved = No font file of { $family } was found
wm-tabs-font-unreadable = lys-wm cannot read the font file of { $family }
wm-tabs-font-file-unreadable = The font file cannot be read
wm-section-workspaces = Workspaces
    .description = Rules for single workspaces, or for every workspace that matches
wm-section-scratchpads = Scratchpads
    .description = Named windows shown and hidden over the others with a shortcut
wm-workspaces = Workspace rules
    .description = A workspace by its number, or every workspace that matches, and how it looks and starts
wm-workspace-number = Workspace { $number }
wm-workspaces-id = Number
wm-workspaces-match = Every workspace
wm-workspaces-name = Name
wm-workspaces-output = Display
wm-workspaces-default = Shown first on its display
wm-workspaces-persistent = Kept while empty
wm-workspaces-on-created-empty = App to open when created empty
wm-workspaces-layout = Layout
wm-workspaces-gaps-inner = Gaps between windows
wm-workspaces-gaps-outer = Gaps at the screen edges
wm-workspaces-borders-width = Border width
wm-workspaces-rounding = Corner rounding
wm-workspaces-shadow = Shadow
wm-scratchpads = Scratchpads
    .description = Each scratchpad's app and size; a shortcut shows or hides it by name
wm-scratchpads-name = Scratchpad name
wm-scratchpads-exec = App
wm-scratchpads-size = Size
wm-scratchpads-hide-on-workspace-switch = Hidden when the workspace changes
wm-size-width = Width %
wm-size-height = Height %
enum-wm-match-any = Any workspace
enum-wm-match-single_tiled = With one tiled window
enum-wm-match-fullscreen = With a fullscreen window
wm-section-animation-events = Window manager animations
    .description = How each of lys-wm's animations moves; an unset value follows the animation above it
wm-section-curves = Animation curves
    .description = Your own easing curves, for the animations to use by name
wm-animations-events = Animations
    .description = Whether each animation plays, its curve, its length and its style
wm-animations-curves = Curves
    .description = A drawn curve or a spring each, named for the animations
wm-animations-curves-name = Curve name
wm-animations-curves-shape = Shape
wm-curve-kind-bezier = Curve
wm-curve-kind-spring = Spring
wm-curve-canvas = Curve shape; drag the two handles to change it
wm-curve-x1 = First handle, time
wm-curve-y1 = First handle, progress
wm-curve-x2 = Second handle, time
wm-curve-y2 = Second handle, progress
wm-curve-stiffness = Stiffness
wm-curve-damping = Damping
wm-curve-mass = Mass
wm-curve-presets = Start from
wm-accel-curve-none = The acceleration profile sets how the pointer speeds up
wm-accel-curve-start = Draw a custom curve
wm-accel-curve-canvas = Pointer speed over device speed; drag a point up or down to change it
wm-accel-curve-step = Device speed between points
wm-accel-curve-add = Add a point
wm-accel-curve-remove = Remove the last point
wm-accel-curve-stop = Use the acceleration profile
wm-curve-own = Your curves
wm-curve-linear = Linear
wm-curve-ease = Ease
wm-curve-ease_in = Ease in
wm-curve-ease_out = Ease out
wm-curve-ease_in_out = Ease in and out
wm-curve-spring = Spring
wm-anim-column-enabled = Plays
wm-anim-column-curve = Curve
wm-anim-column-duration = Length (ms)
wm-anim-column-style = Style
wm-anim-event-global = Everything
wm-anim-event-windows = Windows
wm-anim-event-windows_move = Moving windows
wm-anim-event-windows_in = Opening windows
wm-anim-event-windows_out = Closing windows
wm-anim-event-windows_minimize = Minimizing windows
wm-anim-event-snap = Snapping windows
wm-anim-event-fade = Fades
wm-anim-event-fade_in = Fading in
wm-anim-event-fade_out = Fading out
wm-anim-event-fade_switch = Focus changes
wm-anim-event-fade_dim = Dimming
wm-anim-event-fade_shadow = Shadows
wm-anim-event-fade_snap = Snap preview
wm-anim-event-workspaces = Switching workspaces
wm-anim-event-layers = Panels and overlays
wm-anim-event-layers_in = Panels appearing
wm-anim-event-layers_out = Panels disappearing
wm-anim-event-borderangle = Border gradient
wm-anim-event-zoom = Zoom
wm-anim-event-overview = Overview
wm-anim-event-scratchpad = Scratchpads
wm-anim-style-popin = Pop in
wm-anim-style-slide = Slide
wm-anim-style-slidevert = Slide vertically
wm-anim-style-fade = Fade
wm-anim-style-once = Once
wm-anim-style-loop = Loop
wm-anim-style-morph = Morph
wm-anim-style-elastic = Elastic
wm-anim-style-goo = Goo
wm-anim-style-bud = Bud
wm-anim-style-genie = Genie
wm-app-shortcuts-unreadable = The registered shortcuts cannot be read
wm-devices-reading = Asking lys-wm which devices are connected…
