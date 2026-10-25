import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h, reactive } from 'vue'
import { mount } from '@vue/test-utils'
import ClientDownloadsSettings from '../ClientDownloadsSettings.vue'
import type { ClientDownload } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountEditor(initial: { enabled: boolean; clients: ClientDownload[] }) {
  const form = reactive({ ...initial })
  const Host = defineComponent({
    setup: () => () =>
      h(ClientDownloadsSettings, {
        modelValue: form.clients,
        'onUpdate:modelValue': (v: ClientDownload[]) => (form.clients = v),
        enabled: form.enabled,
        'onUpdate:enabled': (v: boolean) => (form.enabled = v)
      })
  })
  return { wrapper: mount(Host, { global: { stubs: { Icon: true } } }), form }
}

const codex: ClientDownload = {
  id: 'codex',
  name: 'Codex',
  description: '',
  icon_url: '',
  version: '',
  homepage_url: '',
  links: [{ label: 'Windows', platform: 'windows', arch: '', url: 'https://example.com/win' }]
}

describe('ClientDownloadsSettings (CUSTOM)', () => {
  it('edits clients and links in place on the settings form', async () => {
    const { wrapper, form } = mountEditor({ enabled: true, clients: [structuredClone(codex)] })

    await wrapper.get('[data-testid="client-downloads-add-link"]').trigger('click')
    expect(form.clients[0].links).toHaveLength(2)
    expect(form.clients[0].links[1]).toEqual({ label: '', platform: 'windows', arch: '', url: '' })

    await wrapper.get('[data-testid="client-downloads-add-client"]').trigger('click')
    expect(form.clients.map((c) => c.id)).toEqual(['codex', ''])

    const inputs = wrapper.findAll('[data-testid="client-downloads-client"]')[1].findAll('input')
    await inputs[0].setValue('CC Switch')
    expect(form.clients[1].name).toBe('CC Switch')

    await wrapper.findAll('[data-testid="client-downloads-remove-client"]')[0].trigger('click')
    expect(form.clients.map((c) => c.name)).toEqual(['CC Switch'])
  })

  it('hides the list while the page is switched off', async () => {
    const { wrapper, form } = mountEditor({ enabled: false, clients: [structuredClone(codex)] })
    expect(wrapper.find('[data-testid="client-downloads-client"]').exists()).toBe(false)

    await wrapper.get('[data-testid="client-downloads-enabled"]').trigger('click')
    expect(form.enabled).toBe(true)
    expect(wrapper.findAll('[data-testid="client-downloads-client"]')).toHaveLength(1)
  })
})
