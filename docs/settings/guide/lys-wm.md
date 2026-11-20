# lys-wm settings

lys-wm's settings appear when `lys-wm` is on `PATH`, on their own pages or
beside lys-shell's where both set the same thing. They write runtime
overrides to `$XDG_CONFIG_HOME/lys-wm/runtime.yaml` (`~/.config/lys-wm/runtime.yaml`
when unset). lys-wm merges them over `config.yaml`, which this app only reads. See
lys-wm's [config reference](/wm/config) for what each
key does.

| Page | ID | Keys |
| --- | --- | --- |
| Windows | `wm-windows` | `gaps`, `borders`, corner radius, opacity and dimming under `decoration`, `focus`, `fullscreen`, `floating`, `titlebars` |
| Shadow & Blur | `wm-effects` | `decoration.shadow`, `decoration.blur` |
| Motion | `motion` | `animations.enabled` and `animations.reduced_motion` (one switch each with lys-shell's), `overview`, `switcher`, `showdesktop`, `drag_preview`, `hot_corners` |
| Tiling & Tabs | `wm-tiling` | `autotile`, `tabs` |
| Keyboard | `wm-keyboard` | `input.keyboard`, `cursor` |
| Mouse & Touchpad | `wm-pointer` | `input.pointer`, `input.touchpad`, `gestures`, `input.trackpoint` |
| Keyboard Shortcuts | `wm-binds` | `binds`, app shortcuts, `submaps` |
| Displays | `wm-outputs` | Output layout, `primary_output`, `color_management`, `tearing`, `gpu`, `lid_action` |
| Rules & Session | `wm-session` | `rules`, `env`, `xwayland`, `palette` |

## What a change does

1. The override file is read fresh, so hand edits made while the window is open
   are kept.
2. Only the bytes of the key's value change. Comments, blank lines and key order
   stay.
3. The result is written to a temporary file beside it and checked with
   `lys-wm --check-config --runtime <file>`, over the real `config.yaml`.
4. If the file changed during the check, the change starts over, up to three tries.
5. The override file is replaced atomically, keeping its mode.
6. `lys-wmctl reload` makes a running lys-wm read it. With no lys-wm running, the
   page says the change is saved for the next one.

Changes made in quick succession to the same key are written once.

A change lys-wm refuses leaves the file and the session unchanged. The row goes
back to the last accepted value and shows lys-wm's message, with the file and line.
A new or edited card (a bind, rule, output or gesture) is checked before it is
written; while lys-wm refuses it, it stays on the page as an unsaved draft with the
reason. The row's reset button discards a draft.

## Displayed values

A key without an override shows the value lys-wm uses: its built-in config, else
its schema's default. The acceleration profile is one such key per device class:
**Mouse** shows `flat` and **Touchpad** `adaptive` until either is changed. Keys
lys-wm leaves to the system, such as the keyboard layout or a touchpad setting
libinput decides, show **Inherited**. The focus border
defaults to `accent`, which follows the desktop accent set on the **Theme** page.

Without a `config.yaml`, lys-wm's built-in config applies under the overrides, so
its default binds keep working.

## Displays

The Displays page arranges outputs by dragging them on a canvas, and turns each on
or off and sets its resolution, refresh rate and scale, over wlr-output-management.
**Identify** shows each output's name on it.
**Apply** tests the layout, applies it, and asks on every output whether to keep it;
without an answer it reverts after 15 seconds. **Keep** writes the layout to
`outputs`, matching each monitor by its description when known, else by connector.
**Primary display** makes the selected display the one
[`primary_output`](/wm/config#primary_output) names, matched the same way, and
the canvas marks it **Primary**; lys-shell shows its OSD and notifications
there. Only a display that is on can be made primary, and as the key names one
display, making another primary moves the mark.

## Keyboard Shortcuts

Each bind reads as what it does, with its chord as keycaps beside it: the action
lys-wm names with its arguments filled in, such as *Switch to workspace 3* or
*Move window left*, *Launch Firefox* for a command an installed app is started
with, or the bind's own `description` when it has one. An action lys-wm does not
know reads as written and is marked **Unknown action**.

Binds are grouped by what they are for: **Windows**, **Workspaces**, **Layout**,
**Apps**, **Screenshots**, **Media**, **System**, and **Custom** for an action
lys-wm does not know. An app's bind counts as a screenshot or media one by its
key (Print, the volume, media and brightness keys) or by the program it runs
(such as `grim`, `wpctl` or `playerctl`). The search field above finds a bind by
what it does, its group, its action as written or its keys.

A bind's action is picked from lys-wm's actions, grouped the same way, and its
arguments are set by the form lys-wm describes for that action: a direction or
other fixed choice is a row of segments or a dropdown, a workspace a number or a
name, a word such as `follow` a switch, and the command of `exec` the command
picker with its app picker. Arguments outside the action's form stay editable
as text and are marked **Not a form of this action**; an action whose form the
editor does not build (`debug`) takes its arguments as text.

A chord bound twice is marked on every row that holds it, naming the other: two
binds in one list (with **Replace** to keep this one), a bind and a submap's
bind (*Close window in mode resize*; while that mode is on, its bind wins), or an
app shortcut and a bind or submap.

A bind that differs from lys-wm's built-in ones (`lys-wm --default-config`) has a
reset button that puts it back: to the built-in bind it repeats with other flags
or description, else to the built-in bind doing its action on a chord no bind
holds, else to the built-in bind on its chord. A bind of your own, with no
built-in one to go back to, has none.

## Home Manager and Nix

A `config.yaml` linked into the Nix store is never written, so every setting stays
editable. To make changes permanent, copy each key of `runtime.yaml` into the
attribute set Home Manager writes `config.yaml` from, then delete `runtime.yaml`.
`lys-wm --effective-config` lists the keys still coming from it.

## Limits

- **A read-only override file** is not written. The page names it, and the link's
  target when it is a link.
- **A writable link** is written through and stays a link.
- **A dangling link** is an error.
- **YAML anchors and aliases**: an edit through one is refused, naming the line.
  Change it by hand.
- **A file that does not parse** is not written; the page shows the error.
- **Lists and multi-form keys** (`binds`, `rules`, `outputs`, a gradient border) are
  written whole: changing one item overrides the whole value.
