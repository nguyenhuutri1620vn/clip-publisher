import {useEffect, useRef, useState} from 'react';
import {api, errText, VideoSummary} from '../api';

type Props = {video: VideoSummary; onDone: (deleted: boolean) => void};

/** macOS-style sheet asking before a video goes to the Trash. */
export default function ConfirmDelete({video, onDone}: Props) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onDone(false); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onDone]);

  const remove = () => {
    setBusy(true);
    setErr('');
    api.Delete(video.id).then(() => onDone(true)).catch(e => { setErr(errText(e)); setBusy(false); });
  };

  return (
    <div className="sheet-backdrop" onMouseDown={e => { if (e.target === e.currentTarget) onDone(false); }}>
      <div className="sheet" role="alertdialog" aria-labelledby="del-title">
        <div className="sheet-icon" aria-hidden>🗑</div>
        <h3 id="del-title">Chuyển “{video.title || 'video này'}” vào Thùng rác?</h3>
        {video.location ? (
          <p className="dim">
            Thư mục này sẽ vào Thùng rác, khôi phục được từ Thùng rác:
            <span className="path selectable">{video.location}</span>
          </p>
        ) : (
          <p className="dim">Video sẽ được gỡ khỏi thư viện. Không có file nào bị xoá.</p>
        )}
        {err && <div className="banner error">{err}</div>}
        <div className="sheet-actions">
          <button ref={cancelRef} onClick={() => onDone(false)} disabled={busy}>Huỷ</button>
          <button className="destructive" onClick={remove} disabled={busy}>
            {busy ? 'Đang chuyển…' : 'Chuyển vào Thùng rác'}
          </button>
        </div>
      </div>
    </div>
  );
}
