settings-modules-workspaces-min-workspace-count = Minimum Workspaces
    .description = Always show numerically-named workspaces up to this index, even when empty

settings-modules-workspaces-monitor-specific = Monitor Specific
    .description = Show only workspaces on this monitor

settings-modules-workspaces-hide-trailing-empty = Hide Trailing Empty
    .description = Hide niri's auto-allocated empty workspace at the end of each output

settings-modules-workspaces-display-mode = Display Mode
    .description = What identifies each workspace (label, icon, or none)

settings-modules-workspaces-label-strategy = Label Strategy
    .description = How to compose the workspace label from name and index

settings-modules-workspaces-divider = Divider
    .description = Text between workspace identity and app icons

settings-modules-workspaces-urgent-show = Show Urgent
    .description = Pulse animation on workspaces with urgent windows

settings-modules-workspaces-urgent-mode = Urgent Mode
    .description = Pulse the whole workspace or only the urgent app icon

settings-modules-workspaces-window-previews = Window Previews
    .description = Preview a workspace's windows when the pointer rests on it

settings-modules-workspaces-app-icons-show = Show App Icons
    .description = Display window icons per workspace

settings-modules-workspaces-app-icons-dedupe = Deduplicate Icons
    .description = Show one icon per app_id instead of one per window

settings-modules-workspaces-app-icons-fallback = Fallback Icon
    .description = Icon for windows not matched by app-icon-map

settings-modules-workspaces-app-icons-empty = Empty Icon
    .description = Icon shown when a workspace has no application windows

settings-modules-workspaces-icon-gap = Icon Gap
    .description = Spacing between app icons

settings-modules-workspaces-workspace-padding = Workspace Padding
    .description = Padding for content wider than the circle: app icons, a divider or a long name. A short label takes none: this sizes the circle instead

settings-modules-workspaces-icon-size = Icon Size
    .description = Scale multiplier for workspace icons (0.25-3.0)

settings-modules-workspaces-label-size = Label Size
    .description = Scale multiplier for workspace labels (0.25-3.0)

settings-modules-workspaces-workspace-ignore = Ignore Workspaces
    .description = Glob patterns matched against name, index, or id

settings-modules-workspaces-active-indicator = Active Indicator
    .description = Visual style for the active workspace

settings-modules-workspaces-active-color = Active Color
    .description = Color for the active workspace icon and label

settings-modules-workspaces-occupied-color = Occupied Color
    .description = Color for occupied workspace icons and labels

settings-modules-workspaces-empty-color = Empty Color
    .description = Color for empty workspace icons and labels

settings-modules-workspaces-container-bg-color = Container Background
    .description = Background color for the workspaces container

settings-modules-workspaces-border-show = Show Border
    .description = Display border around the workspaces container

settings-modules-workspaces-border-color = Border Color
    .description = Border color for the workspaces container

settings-modules-workspaces-workspace-map = Workspace Map
    .description = Per-workspace icon and color overrides, keyed by name or id

settings-modules-workspaces-app-icon-map = App Icon Map
    .description = Window app_id or title to icon mappings

settings-modules-workspaces-left-click = Left Click
    .description = Action on left click

settings-modules-workspaces-middle-click = Middle Click
    .description = Action on middle click

settings-modules-workspaces-right-click = Right Click
    .description = Action on right click

settings-modules-workspaces-scroll-up = Scroll Up
    .description = Action on scroll up

settings-modules-workspaces-scroll-down = Scroll Down
    .description = Action on scroll down

settings-modules-workspaces-show-special = Show Special
    .description = Include scratchpad workspaces

settings-modules-workspaces-label-use-name = Use Name
    .description = Show workspace name instead of number

settings-modules-workspaces-numbering = Numbering
    .description = How workspace numbers are displayed (absolute or relative)

settings-modules-workspaces-highlight-active-on-other-monitor = Highlight Active on Other Monitor
    .description = Use a different color for active workspaces on other monitors (only applies when { settings-modules-workspaces-monitor-specific } is false)

settings-modules-workspaces-active-on-other-monitor-color = Active on Other Monitor Color
    .description = Color for focused workspace icons and labels active on other monitors (only used when { settings-modules-workspaces-highlight-active-on-other-monitor } is true)

settings-modules-workspaces-hide-empty = Hide Empty
    .description = Hide tags with no clients unless they are active

settings-modules-workspaces-min-tag-count = Minimum Tags
    .description = Always show tags up to this index, even when empty

enum-display-mode-label = Label

enum-display-mode-icon = Icon

enum-display-mode-none = None

enum-active-indicator-background = Background

enum-active-indicator-underline = Underline

enum-label-strategy-index = Index

enum-label-strategy-name-or-index = Name or Index

enum-label-strategy-name-only = Name Only

enum-label-strategy-index-and-name = Index and Name
