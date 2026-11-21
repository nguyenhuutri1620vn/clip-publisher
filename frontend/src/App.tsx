import {useCallback, useEffect, useState} from 'react';
import {api, errText, onVideoChanged, VideoSummary} from './api';
import Library from './components/Library';
import NewVideo from './components/NewVideo';
import Settings from './components/Settings';
import VideoView from './components/VideoView';
import ErrorBoundary from './components/ErrorBoundary';
import ConfirmDelete from './components/ConfirmDelete';

export type View = {kind: 'video'; id: number} | {kind: 'new'} | {kind: 'settings'} | {kind: 'none'};

export default function App() {
  const [videos, setVideos] = useState<VideoSummary[]>([]);
  const [view, setView] = useState<View>({kind: 'none'});
  const [startErr, setStartErr] = useState('');
  const [listErr, setListErr] = useState('');
  const [deleting, setDeleting] = useState<VideoSummary | null>(null);

  const refresh = useCallback(() => {
    api.ListVideos()
      .then(v => { setVideos(v ?? []); setListErr(''); })
      .catch(e => setListErr(errText(e)));
  }, []);

  useEffect(() => {
    api.StartupError().then(setStartErr);
    refresh();
    let t: number | undefined;
    const off = onVideoChanged(() => {
      window.clearTimeout(t);
      t = window.setTimeout(refresh, 150); // imports fire many events at once
    });
    return () => { off(); window.clearTimeout(t); };
  }, [refresh]);

  const selected = view.kind === 'video' ? view.id : null;
  const askDelete = (id: number) => {
    const v = videos.find(x => x.id === id);
    if (v) setDeleting(v);
  };

  // ⌘⌫ moves the selected video to the Trash (Finder's shortcut), unless typing in a field.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.closest('input, textarea, select, [contenteditable]')) return;
      if (e.metaKey && e.key === 'Backspace' && selected !== null) {
        e.preventDefault();
        askDelete(selected);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  return (
    <div className="shell">
      <Library
        videos={videos}
        selected={selected}
        onSelect={id => setView({kind: 'video', id})}
        onNew={() => setView({kind: 'new'})}
        onSettings={() => setView({kind: 'settings'})}
        settingsOpen={view.kind === 'settings'}
        onImport={() => api.ImportExisting().then(refresh).catch(e => setListErr(errText(e)))}
        onDelete={askDelete}
      />
      {deleting && (
        <ConfirmDelete video={deleting} onDone={gone => {
          if (gone && selected === deleting.id) setView({kind: 'none'});
          setDeleting(null);
          if (gone) refresh();
        }}/>
      )}
      <main className="main">
        <ErrorBoundary key={view.kind === 'video' ? `v${view.id}` : view.kind}>
        {startErr && <div className="page"><div className="banner error">App không khởi động được: {startErr}</div></div>}
        {listErr && !startErr && <div className="banner error" style={{margin: 16}}>{listErr}</div>}
        {!startErr && view.kind === 'new' && <NewVideo onCreated={id => { refresh(); setView({kind: 'video', id}); }}/>}
        {!startErr && view.kind === 'settings' && <Settings/>}
        {!startErr && view.kind === 'video' && (
          <VideoView key={view.id} id={view.id} onGone={() => { refresh(); setView({kind: 'none'}); }}
                     onDelete={() => askDelete(view.id)}/>
        )}
        {!startErr && view.kind === 'none' && (
          <div className="empty">
            <div>
              <h2>Chọn một video, hoặc tạo video mới</h2>
              <p>Dán kịch bản theo từng cảnh, Claude sẽ dựng script và app chạy voice + render cho bạn.</p>
              <button className="primary" onClick={() => setView({kind: 'new'})}>Tạo video mới</button>
            </div>
          </div>
        )}
        </ErrorBoundary>
      </main>
    </div>
  );
}
