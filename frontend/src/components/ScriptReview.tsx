import {Fragment, useState} from 'react';
import {api, errText, fmtDuration, SlideDTO, VideoDetail} from '../api';

type Props = {
  d: VideoDetail;
  busy: boolean;
  currentSlide: string | null;
  onSeek: (sec: number) => void;
  onError: (msg: string) => void;
  act: (p: Promise<unknown>) => Promise<unknown>;
};

/** Highlights "[CẦN KIỂM TRA…]" markers so unverified facts are never missed. */
export function Marked({text}: {text: string}) {
  const parts = text.split(/(\[CẦN KIỂM TRA[^\]]*\])/g);
  return <>{parts.map((p, i) => p.startsWith('[CẦN KIỂM TRA')
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
          <h3 className="serif" style={{margin: '22px 0 0'}}>Nguồn</h3>
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
        <div className="slide-meta">
          {s.sceneSummary && <span>Hình: {s.sceneSummary}</span>}
          {s.timeHint && <span>Kịch bản: {s.timeHint}</span>}
          {s.narration && <span>Voice ~{s.estSeconds.toFixed(0)}s</span>}
          {s.claim && <span>Có khẳng định sự thật</span>}
          {s.startSec > 0 || index === 1 ? (
            <button className="link" onClick={() => onSeek(s.startSec)}>Xem cảnh này</button>
          ) : null}
          {!editing && <button className="link" disabled={busy} onClick={open}>Sửa</button>}
        </div>
      </div>
      {s.previewUrl ? <img className="slide-png" src={s.previewUrl} alt={`Khung hình cảnh ${index}`}/> : <div/>}
    </div>
  );
}
