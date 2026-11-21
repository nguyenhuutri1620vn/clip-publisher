import {useCallback, useEffect, useRef, useState} from 'react';
import {api, errText, onLog, onProgress, onVideoChanged, ProgressDTO, VideoDetail} from '../api';
import StatusBadge from './StatusBadge';
import Player from './Player';
import RenderProgress from './RenderProgress';
import ScriptReview from './ScriptReview';
import ChatPanel from './ChatPanel';
import VersionBar from './VersionBar';

type Tab = 'script' | 'chat' | 'versions';

export default function VideoView({id, onGone}: {id: number; onGone: () => void}) {
  const [d, setD] = useState<VideoDetail | null>(null);
  const [progress, setProgress] = useState<ProgressDTO | null>(null);
  const [log, setLog] = useState<string[]>([]);
  const [tab, setTab] = useState<Tab>('script');
  const [err, setErr] = useState('');
  const [menu, setMenu] = useState(false);
  const [currentSlide, setCurrentSlide] = useState<string | null>(null);
  const seekRef = useRef<(sec: number) => void>(() => {});

  const load = useCallback(() => {
    api.GetVideo(id).then(v => {
      setD(v);
      setProgress(v.progress ?? null);
      if (v.logTail?.length) setLog(v.logTail);
    }).catch(e => setErr(errText(e)));
  }, [id]);

  useEffect(() => {
    load();
    const offs = [
      onVideoChanged(changed => { if (changed === id) load(); }),
      onProgress(p => { if (p.videoId === id) setProgress(p); }),
      onLog((vid, line) => { if (vid === id) setLog(l => [...l.slice(-199), line]); }),
    ];
    return () => offs.forEach(off => off());
  }, [id, load]);

  if (!d) return <div className="page dim">{err || 'Đang tải…'}</div>;

  const v = d.video;
  const act = (p: Promise<unknown>) => p.then(() => setErr('')).catch(e => setErr(errText(e)));
  const busy = v.status === 'RENDERING' || v.status === 'GENERATING_SCRIPT';
  const doneRenders = d.renders.filter(r => r.kind === 'render' && r.state === 'DONE');

  return (
    <>
      <header className="video-head">
        <div>
          <h2>{v.title || 'Chưa có tiêu đề'}</h2>
          <div className="row-meta">
            <StatusBadge status={v.status} queuePos={v.queuePos}/>
            {d.currentVersion > 0 && <span>Script v{d.currentVersion}</span>}
            {d.voice && <span>Giọng {d.voice}</span>}
            <span>{d.music ? `Nhạc ${d.music}` : 'Không nhạc'}</span>
            {!v.editable && <span className="tag">Chỉ xem</span>}
          </div>
        </div>
        <div className="actions">
          {busy && <button onClick={() => act(api.Cancel(id))}>Huỷ</button>}
          <div className="menu">
            <button onClick={() => setMenu(m => !m)} aria-expanded={menu}>Thêm</button>
            {menu && (
              <div className="menu-pop" onMouseLeave={() => setMenu(false)}>
                <button onClick={() => { setMenu(false); act(api.OpenFolder(id)); }}>Mở thư mục video</button>
                <button onClick={() => { setMenu(false); act(api.Hide(id)).then(onGone); }}>Ẩn khỏi thư viện</button>
                {v.origin === 'app' && (
                  <button className="danger" onClick={() => {
                    setMenu(false);
                    if (confirm('Chuyển thư mục video này vào Thùng rác? Có thể khôi phục từ Thùng rác.')) {
                      act(api.Delete(id)).then(onGone);
                    }
                  }}>Xoá (vào Thùng rác)</button>
                )}
              </div>
            )}
          </div>
        </div>
      </header>

      <div className="video-body">
        <section className="stage">
          {err && <div className="banner error">{err}</div>}
          {v.status === 'FAILED' && (
            <div className="banner error">
              {v.statusDetail || 'Có lỗi.'}
              <div style={{marginTop: 8}}><button onClick={() => act(api.Retry(id))}>Thử lại</button></div>
            </div>
          )}
          {v.status === 'GENERATING_SCRIPT' && <RenderProgress kind="claude" progress={progress} log={[]}/>}
          {v.status === 'RENDERING' && <RenderProgress kind="render" progress={progress} log={log}/>}
          {v.status !== 'RENDERING' && v.status !== 'GENERATING_SCRIPT' && (
            <Player renders={doneRenders} slides={d.slides} currentVersion={d.currentVersion} poster={v.thumbUrl}
                    onSlide={setCurrentSlide} seekRef={seekRef}/>
          )}
          {v.status === 'FAILED' && log.length > 0 && (
            <details>
              <summary className="dim small">Log lần chạy cuối</summary>
              <div className="log">{log.join('\n')}</div>
            </details>
          )}
          {!v.editable && <div className="banner info">Video này không có script.json trong final/, nên chỉ xem được.</div>}
        </section>

        <section className="side">
          {v.editable && (
            <nav className="tabs"><div className="tabs-inner">
              <button className={tab === 'script' ? 'on' : ''} onClick={() => setTab('script')}>Script</button>
              <button className={tab === 'chat' ? 'on' : ''} onClick={() => setTab('chat')}>Chat với Claude</button>
              <button className={tab === 'versions' ? 'on' : ''} onClick={() => setTab('versions')}>
                Phiên bản ({d.versions.length})
              </button>
            </div></nav>
          )}
          {v.editable && tab === 'script' && (
            <ScriptReview d={d} busy={busy} currentSlide={currentSlide} onSeek={s => seekRef.current(s)}
                          onError={setErr} act={act}/>
          )}
          {v.editable && tab === 'chat' && <ChatPanel d={d} busy={busy} act={act}/>}
          {v.editable && tab === 'versions' && <VersionBar d={d} busy={busy} act={act}/>}
        </section>
      </div>
    </>
  );
}
