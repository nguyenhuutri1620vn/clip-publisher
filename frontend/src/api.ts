import * as App from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {config, photos, pipeline, service, scriptmodel, workspace} from '../wailsjs/go/models';

export type VideoSummary = service.VideoSummary;
export type VideoDetail = service.VideoDetail;
export type SlideDTO = service.SlideDTO;
export type RenderDTO = service.RenderDTO;
export type VersionDTO = service.VersionDTO;
export type ProgressDTO = service.ProgressDTO;
export type Warning = scriptmodel.Warning;
export type SlideChange = scriptmodel.SlideChange;
export type ChatMessage = workspace.ChatMessage;
export type Settings = config.Settings;
export type Check = pipeline.Check;
export type Candidate = photos.Candidate;
export type PhotoRef = photos.Ref;

export const api = App;

export function onVideoChanged(cb: (id: number) => void): () => void {
  return EventsOn('video:changed', (d: {id: number}) => cb(d.id));
}

export function onProgress(cb: (p: ProgressDTO) => void): () => void {
  return EventsOn('job:progress', cb);
}

export function onLog(cb: (videoId: number, line: string) => void): () => void {
  return EventsOn('job:log', (d: {videoId: number; line: string}) => cb(d.videoId, d.line));
}

export function errText(e: unknown): string {
  if (typeof e === 'string') return e;
  if (e instanceof Error) return e.message;
  return String(e);
}

export function fmtDuration(sec: number): string {
  if (!sec || sec <= 0) return '';
  const s = Math.round(sec);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function fmtRelative(iso: string): string {
  const t = new Date(iso).getTime();
  if (!t) return '';
  const m = Math.round((Date.now() - t) / 60000);
  if (m < 1) return 'vừa xong';
  if (m < 60) return `${m} phút trước`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} giờ trước`;
  return new Date(iso).toLocaleDateString('vi-VN');
}

export const statusLabel: Record<string, string> = {
  DRAFT: 'Nháp',
  GENERATING_SCRIPT: 'Claude đang viết…',
  SCRIPT_REVIEW: 'Chờ duyệt script',
  RENDERING: 'Đang render',
  READY: 'Sẵn sàng',
  FAILED: 'Lỗi',
};
