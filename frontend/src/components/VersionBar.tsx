import {api, VideoDetail} from '../api';

const author: Record<string, string> = {claude: 'Claude', manual: 'Sửa tay', restore: 'Khôi phục', import: 'Import'};
const fieldName: Record<string, string> = {title: 'tiêu đề', subtitle: 'phụ đề', narration: 'lời đọc', scene: 'hình', other: 'khác'};

type Props = {d: VideoDetail; busy: boolean; act: (p: Promise<unknown>) => Promise<unknown>};

export default function VersionBar({d, busy, act}: Props) {
  const versions = [...d.versions].reverse();
  return (
    <div className="tab-body">
      {d.changes.length > 0 && (
        <>
          <p className="dim small" style={{marginTop: 0}}>v{d.currentVersion} so với v{d.currentVersion - 1}:</p>
          <div className="chips" style={{marginBottom: 16}}>
            {d.changes.map(c => (
              <span key={c.id + c.kind} className={`chip ${c.kind}`}>
                cảnh {c.id}: {c.kind === 'added' ? 'thêm mới' : c.kind === 'removed' ? 'đã xoá'
                  : (c.fields ?? []).map(f => fieldName[f] ?? f).join(', ')}
              </span>
            ))}
          </div>
        </>
      )}
      <div className="versions">
        {versions.map(v => (
          <div key={v.n} className="version">
            <span className="n">v{v.n}</span>
            <div>
              <div>{v.summary || author[v.author] || v.author}</div>
              <div className="dim small">{author[v.author] ?? v.author} · {v.createdAt}</div>
            </div>
            {v.n === d.currentVersion
              ? <span className="small dim">Đang dùng</span>
              : <button disabled={busy} onClick={() => act(api.RestoreVersion(d.video.id, v.n))}>Khôi phục bản này</button>}
          </div>
        ))}
      </div>
    </div>
  );
}
