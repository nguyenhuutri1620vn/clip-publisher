import {useMemo, useState} from 'react';
import {fmtDuration, fmtRelative, VideoSummary} from '../api';
import StatusBadge from './StatusBadge';

type Props = {
  videos: VideoSummary[];
  selected: number | null;
  onSelect: (id: number) => void;
  onNew: () => void;
  onSettings: () => void;
  settingsOpen: boolean;
  onImport: () => void;
};

function fold(s: string): string {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/gi, 'd').toLowerCase();
}

export default function Library({videos, selected, onSelect, onNew, onSettings, settingsOpen, onImport}: Props) {
  const [q, setQ] = useState('');
  const shown = useMemo(() => {
    const f = fold(q.trim());
    return f ? videos.filter(v => fold(v.title).includes(f)) : videos;
  }, [videos, q]);

  return (
    <aside className="sidebar">
      <div className="sidebar-head">
        <h1>Clip Publisher</h1>
        <button className="primary" onClick={onNew}>+ Video mới</button>
        <input type="text" placeholder="Tìm theo tiêu đề" value={q} onChange={e => setQ(e.target.value)}/>
      </div>
      <div className="list">
        {shown.map(v => (
          <div key={v.id} className={`row ${v.id === selected ? 'active' : ''}`} onClick={() => onSelect(v.id)}
               role="button" tabIndex={0} onKeyDown={e => e.key === 'Enter' && onSelect(v.id)}>
            <div className="thumb" style={{backgroundImage: `url(${v.thumbUrl})`}}/>
            <div>
              <div className="row-title">{v.title || 'Chưa có tiêu đề'}</div>
              <div className="row-meta">
                <StatusBadge status={v.status} queuePos={v.queuePos}/>
                {v.duration > 0 && <span>{fmtDuration(v.duration)}</span>}
                <span>{fmtRelative(v.updatedAt)}</span>
                {!v.editable && <span className="tag">Chỉ xem</span>}
              </div>
            </div>
          </div>
        ))}
        {videos.length === 0 && (
          <p className="dim small" style={{padding: '8px 6px'}}>
            Chưa có video. Bấm “+ Video mới”, hoặc “Quét video cũ” để lấy các video trong make-video-free/output.
          </p>
        )}
      </div>
      <div className="sidebar-foot">
        <button onClick={onImport} title="Thêm các video mới trong make-video-free/output">Quét video cũ</button>
        <button onClick={onSettings} className={settingsOpen ? 'primary' : ''}>Cài đặt</button>
      </div>
    </aside>
  );
}
