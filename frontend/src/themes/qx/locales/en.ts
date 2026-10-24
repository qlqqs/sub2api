// CUSTOM: QxAPI theme copy (imported by i18n/locales/en/index.ts).
import nav from '../shell/locales/en'
import dashboard from '../dashboard/locales/en'
import studio from '../studio/locales/en'
export default {
  qxTheme: {
    nav,
    dashboard,
    studio,
    listSeparator: ', ',
    hero: {
      description: 'Create an API key after signing up, then enter the base URL in {tools} or similar tools to call the models. Usage is deducted from your balance.',
      register: 'Sign up',
      models: 'Models',
      protocols: 'Protocols',
      billing: 'Billing',
      billingValue: 'By token usage'
    },
    console: {
      endpoint: 'Base URL',
      copy: 'Copy',
      copied: 'Copied',
      keyHint: 'Replace $API_KEY with a key created in the console',
      prompt: 'Draft an outline for my lab report'
    },
    pricing: {
      kicker: 'PRICING',
      title: 'Model pricing',
      lead: 'Same as the model plaza, per million tokens, deducted from your balance.',
      model: 'Model',
      context: 'Context',
      input: 'Input',
      output: 'Output',
      cacheRead: 'Cache read',
      perImage: '/ image',
      perRequest: '/ request',
      timePricingNote: 'Some models are priced differently at certain hours. See the model plaza for details.',
      more: 'See full pricing, including cache writes, in the model plaza'
    },
    steps: {
      kicker: 'QUICK START',
      title: 'Get started in three steps',
      register: {
        title: 'Sign up and top up',
        description: 'Register with your email and add balance in the console'
      },
      key: {
        title: 'Create an API key',
        description: 'Create a key on the API Keys page'
      },
      ccs: {
        label: 'One-click import',
        badge: 'Recommended',
        title: 'CC Switch',
        description: 'Click "Import to CCS" next to your key on the API Keys page to fill in the base URL and key',
        keys: 'Go to API Keys',
        download: 'Download CC Switch'
      },
      manual: {
        label: 'Manual setup',
        title: 'Base URL',
        description: 'Enter the matching address and your key in your tool'
      },
      paths: 'Endpoints'
    }
  }
}
