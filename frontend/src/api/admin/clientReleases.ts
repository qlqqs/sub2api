/**
 * CUSTOM: QxAPI 客户端版本发布。接口约定见
 * .trellis/tasks/10-30-client-releases/design.md「管理接口返回格式」。
 */
import type { AxiosProgressEvent } from 'axios'
import { apiClient } from '../client'

export type ClientReleaseStatus = 'draft' | 'published' | 'withdrawn'

export interface ClientReleaseFile {
  name: string
  size: number
  sha256: string
  uploaded: boolean
}

export interface ClientRelease {
  id: number
  product: string
  version: string
  notes: string
  status: ClientReleaseStatus
  published_at: string | null
  created_at: string
  updated_at: string
  files: ClientReleaseFile[]
}

export interface ClientReleasePublishResult {
  release: ClientRelease
  removed_versions: string[]
}

export async function list(): Promise<ClientRelease[]> {
  const { data } = await apiClient.get<ClientRelease[]>('/admin/client-releases')
  return data
}

/** manifest 是 release.json 的原文。 */
export async function create(manifest: string): Promise<ClientRelease> {
  const { data } = await apiClient.post<ClientRelease>('/admin/client-releases', manifest, {
    headers: { 'Content-Type': 'application/json' }
  })
  return data
}

export async function checkFile(id: number, name: string): Promise<void> {
  await apiClient.post(`/admin/client-releases/${id}/files/${encodeURIComponent(name)}/check`)
}

export async function uploadFile(
  id: number,
  file: File,
  onProgress: (loaded: number) => void
): Promise<ClientRelease> {
  const { data } = await apiClient.put<ClientRelease>(
    `/admin/client-releases/${id}/files/${encodeURIComponent(file.name)}`,
    file,
    {
      headers: { 'Content-Type': 'application/octet-stream' },
      timeout: 0,
      onUploadProgress: (event: AxiosProgressEvent) => onProgress(event.loaded)
    }
  )
  return data
}

export async function updateNotes(id: number, notes: string): Promise<ClientRelease> {
  const { data } = await apiClient.put<ClientRelease>(`/admin/client-releases/${id}/notes`, { notes })
  return data
}

export async function publish(id: number): Promise<ClientReleasePublishResult> {
  const { data } = await apiClient.post<ClientReleasePublishResult>(`/admin/client-releases/${id}/publish`)
  return data
}

export async function withdraw(id: number): Promise<ClientRelease> {
  const { data } = await apiClient.post<ClientRelease>(`/admin/client-releases/${id}/withdraw`)
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/client-releases/${id}`)
}

export default { list, create, checkFile, uploadFile, updateNotes, publish, withdraw, remove }
