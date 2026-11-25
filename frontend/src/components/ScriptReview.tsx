import {Fragment, useState} from 'react';
import {api, Candidate, errText, fmtDuration, SlideDTO, VideoDetail} from '../api';

type Props = {
  d: VideoDetail;
  busy: boolean;
  currentSlide: string | null;
  onSeek: (sec: number) => void;
  onError: (msg: string) => void;
  act: (p: Promise<unknown>) => Promise<unknown>;
};

/** Highlights "[CẦN KIỂM TRA…]" / "[CẦN BỔ SUNG…]" markers so unverified facts are never missed. */
export function Marked({text}: {text: string}) {
  const parts = text.split(/(\[\s*cần (?:kiểm tra|bổ sung)[^\]]*\])/giu);
  return <>{parts.map((p, i) => /^\[\s*cần (kiểm tra|bổ sung)/iu.test(p)
    ? <span key={i} className="check">{p}</span>
    : <Fragment key={i}>{p}</Fragment>)}</>;
}

export default function ScriptReview({d, busy, currentSlide, onSeek, act}: Props) {
  const previewRunning = d.renders.some(r => r.kind === 'preview' && r.state === 'RUNNING');
  const hasRender = d.renders.some(r => r.kind === 'render' && r.state === 'DONE' && r.version === d.currentVersion);
  const scriptWarnings = d.warnings.filter(w => !w.slideId);
  const slideWarnings = (id: string) => d.warnings.filter(w => w.slideId === id);
  const est = d.slides.reduce((t, s) => t + s.estSeconds + 0.8, 0);

  if (d.slides.length === 0) {
    return <div className="tab-body dim">Chưa có script.</div>;
  }

  return (
    <div className="tab-body">
      <div className="toolbar">
        <span className="dim small">{d.slides.length} cảnh · ước tính {fmtDuration(est)}</span>
        <span className="grow"/>
        <button disabled={busy || previewRunning} onClick={() => act(api.Preview(d.video.id))}>
          {previewRunning ? 'Đang vẽ khung hình…' : 'Xem trước khung hình'}
        </button>
        <button className="primary" disabled={busy} onClick={() => act(api.Render(d.video.id))}>
          {hasRender ? 'Render lại' : 'Render video'}
        </button>
      </div>

      {scriptWarnings.map((w, i) => <div key={i} className="banner warn">{w.message}</div>)}

      {d.slides.map((s, i) => (
        <SlideCard key={s.id} index={i + 1} s={s} videoId={d.video.id} busy={busy}
                   warnings={slideWarnings(s.id).map(w => w.message)}
                   current={s.id === currentSlide} onSeek={onSeek} act={act}/>
      ))}

      {d.sources.length > 0 && (
        <>
          <h3 className="section-title">Nguồn</h3>
          <ol className="sources selectable">
            {d.sources.map((src, i) => <li key={i}><Marked text={src.text}/></li>)}
          </ol>
        </>
      )}
    </div>
  );
}

type CardProps = {
  index: number; s: SlideDTO; videoId: number; busy: boolean; warnings: string[]; current: boolean;
  onSeek: (sec: number) => void; act: (p: Promise<unknown>) => Promise<unknown>;
};

function SlideCard({index, s, videoId, busy, warnings, current, onSeek}: CardProps) {
  const [editing, setEditing] = useState(false);
  const [raw, setRaw] = useState(false);
  const [title, setTitle] = useState(s.title);
  const [subtitle, setSubtitle] = useState(s.subtitle);
  const [narration, setNarration] = useState(s.narration);
  const [json, setJson] = useState(s.rawJson);
  const [err, setErr] = useState('');
  const [saving, setSaving] = useState(false);
  const [photoOpen, setPhotoOpen] = useState(false);
  const [photoQ, setPhotoQ] = useState((s.photoQueries ?? [])[0] ?? '');
  const [cands, setCands] = useState<Candidate[] | null>(null);
  const [photoBusy, setPhotoBusy] = useState(false);
  const [photoErr, setPhotoErr] = useState('');

  const find = () => {
    const q = photoQ.trim();
    if (!q) { setPhotoErr('Hãy nhập từ khóa'); return; }
    setPhotoBusy(true); setPhotoErr('');
    api.FindPhotos(videoId, q)
      .then(setCands)
      .catch(e => { setCands(null); setPhotoErr(errText(e)); })
      .finally(() => setPhotoBusy(false));
  };
  const pick = (c: Candidate) => {
    setPhotoBusy(true); setPhotoErr('');
    // Success also emits video:changed, which reloads the card with the new version and credit.
    api.PickPhoto(videoId, s.id, c)
      .then(() => { setPhotoOpen(false); setCands(null); })
      .catch(e => setPhotoErr(errText(e)))
      .finally(() => setPhotoBusy(false));
  };

  const open = () => {
    setTitle(s.title); setSubtitle(s.subtitle); setNarration(s.narration);
    setJson(JSON.stringify(JSON.parse(s.rawJson), null, 2));
    setErr(''); setEditing(true);
  };
  const save = () => {
    setSaving(true);
    const p = raw ? api.EditSlideRaw(videoId, s.id, json) : api.EditSlideText(videoId, s.id, title, subtitle, narration);
    p.then(() => setEditing(false)).catch(e => setErr(errText(e))).finally(() => setSaving(false));
  };

  return (
    <div className={`slide ${current ? 'current' : ''}`}>
      <div className="slide-index">{index}</div>
      <div>
        {!editing ? (
          <>
            <div className="slide-title selectable"><Marked text={s.title}/></div>
            {s.subtitle && <div className="slide-sub selectable"><Marked text={s.subtitle}/></div>}
            {s.narration && <div className="slide-narr selectable"><Marked text={s.narration}/></div>}
          </>
        ) : (
          <div className="slide-edit">
            {!raw ? (
              <>
                <input type="text" value={title} onChange={e => setTitle(e.target.value)} aria-label="Chữ trên hình (tiêu đề)"/>
                <input type="text" value={subtitle} onChange={e => setSubtitle(e.target.value)} aria-label="Phụ đề"/>
                <textarea value={narration} onChange={e => setNarration(e.target.value)} aria-label="Lời đọc"/>
              </>
            ) : (
              <textarea className="json" value={json} onChange={e => setJson(e.target.value)} spellCheck={false}/>
            )}
            {err && <div className="banner error">{err}</div>}
            <div className="toolbar">
              <button className="primary" disabled={saving} onClick={save}>{saving ? 'Đang kiểm tra…' : 'Lưu thành bản mới'}</button>
              <button onClick={() => setEditing(false)}>Huỷ</button>
              <span className="grow"/>
              <button className="link" onClick={() => setRaw(r => !r)}>{raw ? 'Sửa chữ' : 'JSON nâng cao'}</button>
            </div>
          </div>
        )}
        {s.note && <div className="note"><Marked text={s.note}/></div>}
        {warnings.map((w, i) => <div key={i} className="note warn">{w}</div>)}
        {photoOpen && (
          <div className="photo-panel">
            <div className="photo-search">
              <input type="text" value={photoQ} onChange={e => setPhotoQ(e.target.value)}
                     onKeyDown={e => { if (e.key === 'Enter') find(); }} aria-label="Từ khóa tìm ảnh"/>
              <button disabled={photoBusy} onClick={find}>{photoBusy && !cands ? 'Đang tìm…' : 'Tìm'}</button>
            </div>
            {(s.photoQueries ?? []).length > 0 && (
              <div className="photo-hints small dim">
                Gợi ý: {(s.photoQueries ?? []).map(q => <button key={q} className="link" onClick={() => setPhotoQ(q)}>{q}</button>)}
              </div>
            )}
            {photoErr && <div className="banner error">{photoErr}</div>}
            {cands && cands.length === 0 && (
              <div className="dim small">Không tìm thấy ảnh cho “{photoQ}”. Thử từ khóa khác.</div>
            )}
            {cands && cands.length > 0 && (
              <div className="photo-grid">
                {cands.map(c => (
                  <figure key={c.pageUrl} className="photo-card">
                    <img src={c.thumbUrl} alt={c.title}/>
                    <figcaption className="small">{c.author || 'không rõ tác giả'} · {c.license}</figcaption>
                    <button disabled={photoBusy} onClick={() => pick(c)}>Chọn</button>
                  </figure>
                ))}
              </div>
            )}
          </div>
        )}
        {(s.photos ?? []).map(p => <div key={p.sha256} className="slide-credit small dim">{p.credit}</div>)}
        <div className="slide-meta">
          {s.sceneSummary && <span>Hình: {s.sceneSummary}</span>}
          {s.timeHint && <span>Kịch bản: {s.timeHint}</span>}
          {s.narration && <span>Voice ~{s.estSeconds.toFixed(0)}s</span>}
          {s.claim && <span>Có khẳng định sự thật</span>}
          {s.startSec > 0 || index === 1 ? (
            <button className="link" onClick={() => onSeek(s.startSec)}>Xem cảnh này</button>
          ) : null}
          {!editing && <button className="link" disabled={busy} onClick={() => setPhotoOpen(o => !o)}>Tìm ảnh</button>}
          {!editing && <button className="link" disabled={busy} onClick={open}>Sửa</button>}
        </div>
      </div>
      {s.previewUrl ? <img className="slide-png" src={s.previewUrl} alt={`Khung hình cảnh ${index}`}/> : <div/>}
    </div>
  );
}
