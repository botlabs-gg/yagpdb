import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Goblox Documentation",
  description: "Documentation for the Goblox Go library.",

  head: [
    ['link', { rel: 'icon', href: '/goblox_icon.png' }],
  ],

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    siteTitle: "Goblox",
    logo: '/goblox_icon.png',

    search: {
      provider: 'local',
    },
    outline: [2, 4],
    
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Introduction', link: '/getting-started/welcome' },
      { text: 'Guides', link: '/guides/opencloud/authentication' },
      { text: 'Documentation', link: '/documentation/opencloud/common' },
    ],

    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'About Goblox', link: '/getting-started/welcome' },
        ],
      },
      {
        text: 'Guides',
        items: [
          {
            text: 'OpenCloud',
            items: [
              { text: "Authentication", link: '/guides/opencloud/authentication' },
              { text: 'Pagination', link: '/guides/opencloud/pagination' },
              { text: "Luau Execution", link: '/guides/opencloud/luau-execution' },
            ]
          }
        ]
      },
      {
        text: "Documentation",
        items: [
          {
            text: "OpenCloud",
            items: [
              { text: "Common", link: "/documentation/opencloud/common" },
              { text: 'Luau Execution', link: '/documentation/opencloud/luau-execution' },
            ]
          }
        ]
      },
      {
        text: 'Packages',
        items: [
          { text: 'methodutil', link: '/packages/methodutil' },
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/typical-developers/goblox' },
      { icon: 'discord', link: 'https://discord.gg/typical' },
      { icon: 'twitter', link: 'https://twitter.com/typicaldevelops' },
      { icon: 'roblox', link: 'https://www.roblox.com/communities/2700233/Typical-Developers' },
    ]
  }
})
