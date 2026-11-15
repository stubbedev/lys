import { readdirSync } from 'node:fs'
import { defineConfig } from 'vitepress'

const repo = 'https://github.com/stubbedev/lys'

const shellConfigPages = (dir, skip = []) =>
  readdirSync(new URL(`../shell/config/${dir}`, import.meta.url))
    .filter((f) => f.endsWith('.md'))
    .map((f) => f.slice(0, -3))
    .filter((name) => !skip.includes(name))
    .sort()
    .map((name) => ({ text: name, link: `/config/${dir}${name}` }))

const install = {
  text: 'Install',
  items: [
    { text: 'Overview', link: '/install/' },
    { text: 'NixOS', link: '/install/nixos' },
    { text: 'Debian and Ubuntu', link: '/install/debian' },
    { text: 'Fedora', link: '/install/fedora' },
    { text: 'Other systems', link: '/install/other' },
  ],
}

const sections = [
  {
    dir: 'shell',
    name: 'lys-shell',
    blurb: 'Desktop shell',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Editing config', link: '/guide/editing-config' },
          { text: 'Bars and layouts', link: '/guide/bars-and-layouts' },
          { text: 'Themes', link: '/guide/themes' },
          { text: 'Custom styles', link: '/guide/custom-styles' },
          { text: 'Custom icons', link: '/guide/custom-icons' },
          { text: 'Custom modules', link: '/guide/custom-modules' },
          { text: 'Launcher', link: '/guide/launcher' },
          { text: 'CLI', link: '/guide/cli' },
        ],
      },
      {
        text: 'Config',
        collapsed: true,
        items: [
          { text: 'Overview', link: '/config/' },
          { text: 'Types', link: '/config/types' },
        ],
      },
      { text: 'Top-level sections', collapsed: true, items: shellConfigPages('', ['index', 'types']) },
      { text: 'Bar modules', collapsed: true, items: shellConfigPages('modules/') },
    ],
  },
  {
    dir: 'wm',
    name: 'lys-wm',
    blurb: 'Compositor',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Install', link: '/guide/install' },
          { text: 'Scope', link: '/guide/scope' },
          { text: 'Upgrading in place', link: '/guide/upgrade' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Config', link: '/config' },
          { text: 'Actions', link: '/actions' },
          { text: 'IPC', link: '/ipc' },
          { text: 'Screen shaders', link: '/shaders' },
        ],
      },
      {
        text: 'Contributors',
        items: [
          { text: 'Contributing', link: '/guide/contributing' },
          { text: 'Architecture', link: '/architecture' },
          { text: 'Frame pacing', link: '/frame-pacing' },
        ],
      },
    ],
  },
  {
    dir: 'login',
    name: 'lys-login',
    blurb: 'Display manager and lock screen',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Configuration', link: '/guide/configuration' },
          { text: 'PAM services', link: '/guide/pam' },
          { text: 'The lock screen', link: '/guide/lock-screen' },
        ],
      },
      {
        text: 'Install',
        items: [
          { text: 'NixOS', link: '/install/nixos' },
          { text: 'Debian, Ubuntu and Fedora', link: '/install/packages' },
          { text: 'Other systems', link: '/install/other' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Architecture', link: '/reference/architecture' },
          { text: 'Command line', link: '/reference/cli' },
          { text: 'Development', link: '/reference/development' },
        ],
      },
    ],
  },
  {
    dir: 'settings',
    name: 'lys-settings',
    blurb: 'Settings app',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Install', link: '/guide/install' },
          { text: 'lys-wm settings', link: '/guide/lys-wm' },
          { text: 'How it works', link: '/guide/how-it-works' },
          { text: 'Development', link: '/guide/development' },
        ],
      },
    ],
  },
  {
    dir: 'polkit',
    name: 'lys-polkit',
    blurb: 'Polkit agent',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Install', link: '/guide/install' },
          { text: 'Configuration', link: '/guide/configuration' },
          { text: 'How it works', link: '/guide/how-it-works' },
          { text: 'Development', link: '/guide/development' },
        ],
      },
    ],
  },
  {
    dir: 'secrets',
    name: 'lys-secrets',
    blurb: 'Secret Service',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Configuration', link: '/guide/configuration' },
          { text: 'How it works', link: '/guide/how-it-works' },
          { text: 'How secrets are protected', link: '/guide/security' },
          { text: 'Development', link: '/guide/development' },
        ],
      },
    ],
  },
  {
    dir: 'splash',
    name: 'lys-splash',
    blurb: 'Boot splash',
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Install', link: '/guide/install' },
          { text: 'Configuration and themes', link: '/guide/configuration' },
          { text: 'Command line', link: '/guide/command-line' },
        ],
      },
      {
        text: 'Internals',
        items: [
          { text: 'How it works', link: '/guide/how-it-works' },
          { text: 'Development', link: '/guide/development' },
        ],
      },
    ],
  },
  {
    dir: 'lib',
    name: 'lys-lib',
    blurb: 'Building blocks',
    library: true,
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Overview and package map', link: '/overview' },
          { text: 'Development', link: '/development' },
        ],
      },
      {
        text: 'Packages',
        items: [
          { text: 'Rendering and GPU', link: '/rendering' },
          { text: 'Wayland', link: '/wayland' },
          { text: 'Media', link: '/media' },
          { text: 'System services', link: '/system' },
          { text: 'xkb', link: '/xkb' },
        ],
      },
      {
        text: 'Third party',
        collapsed: true,
        items: [
          { text: 'Patch policy', link: '/third-party/' },
          { text: 'godbus', link: '/third-party/godbus' },
          { text: 'gomath', link: '/third-party/gomath' },
          { text: 'oksvg', link: '/third-party/oksvg' },
          { text: 'rasterx', link: '/third-party/rasterx' },
          { text: 'typesetting', link: '/third-party/typesetting' },
          { text: 'vector', link: '/third-party/vector' },
          { text: 'vk', link: '/third-party/vk' },
          { text: 'wayland', link: '/third-party/wayland' },
        ],
      },
    ],
  },
  {
    dir: 'ui',
    name: 'lys-ui',
    blurb: 'Widget toolkit',
    library: true,
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Widgets', link: '/widgets' },
          { text: 'Demos', link: '/guide/demos' },
          { text: 'Development', link: '/guide/development' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Architecture', link: '/architecture' },
          { text: 'Application model', link: '/application-model' },
          { text: 'Components', link: '/components' },
          { text: 'Threading', link: '/threading' },
          { text: 'Input model', link: '/input-model' },
          { text: 'CSS', link: '/css' },
          { text: 'Appearance', link: '/appearance' },
          { text: 'Icons', link: '/icons' },
          { text: 'Accessibility', link: '/a11y' },
          { text: 'Inspector', link: '/inspector' },
          { text: 'Completeness', link: '/completeness' },
        ],
      },
    ],
  },
  {
    dir: 'desktop',
    name: 'lys-desktop',
    blurb: 'Shared desktop packages',
    library: true,
    sidebar: [],
  },
]

const sectionSidebar = ({ dir, name, sidebar }) => [
  { text: name, items: [{ text: 'Overview', link: `/${dir}/` }] },
  ...sidebar.map((group) => ({
    ...group,
    items: group.items.map((item) => ({ ...item, link: `/${dir}${item.link}` })),
  })),
]

const menu = (text, list) => ({
  text,
  items: list.map(({ dir, name, blurb }) => ({ text: `${name}: ${blurb}`, link: `/${dir}/` })),
  activeMatch: `^/(${list.map(({ dir }) => dir).join('|')})/`,
})

const apps = sections.filter((s) => !s.library)
const libraries = sections.filter((s) => s.library)

export default defineConfig({
  title: 'lys',
  description: 'A Wayland desktop in pure Go: shell, compositor, display manager, settings and the services around them.',
  base: '/lys/',
  cleanUrls: true,
  lastUpdated: true,
  srcExclude: ['node_modules/**'],
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: '/lys/lys.svg' }]],
  markdown: { theme: { light: 'github-light', dark: 'tokyo-night' } },
  vite: { ssr: { noExternal: ['lys-docs-theme'] } },
  themeConfig: {
    logo: '/lys.svg',
    nav: [
      { text: 'Install', link: '/install/', activeMatch: '^/install/' },
      menu('Apps', apps),
      menu('Libraries', libraries),
    ],
    sidebar: {
      '/install/': [install],
      ...Object.fromEntries(sections.map((s) => [`/${s.dir}/`, sectionSidebar(s)])),
    },
    socialLinks: [{ icon: 'github', link: repo }],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright 2026 lys contributors',
    },
    editLink: { pattern: `${repo}/edit/main/docs/:path`, text: 'Edit this page on GitHub' },
    outline: { level: [2, 3] },
    search: { provider: 'local' },
  },
})
