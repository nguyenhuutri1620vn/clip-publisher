import {MutableRefObject, useEffect, useRef, useState} from 'react';
import {RenderDTO, SlideDTO} from '../api';

type Props = {
  renders: RenderDTO[];
  slides: SlideDTO[];
  currentVersion: number;
  poster: string;
  onSlide: (id: string | null) => void;
  seekRef: MutableRefObject<(sec: number) => void>;
};

export default function Player({renders, slides, currentVersion, poster, onSlide, seekRef}: Props) {
  const [renderId, setRenderId] = useState<number | null>(renders[0]?.id ?? null);
  const [loop, setLoop] = useState(false);
  const [rate, setRate] = useState(1);
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    if (!renders.some(r => r.id === renderId)) setRenderId(renders[0]?.id ?? null);
  }, [renders, renderId]);
  useEffect(() => { if (ref.current) ref.current.playbackRate = rate; }, [rate, renderId]);

  const current = renders.find(r => r.id === renderId);
  const timed = current && current.version === currentVersion && slides.some(s => s.startSec > 0);

  seekRef.current = (sec: number) => {
    if (ref.current && timed) {
      ref.current.currentTime = sec + 0.05;
      ref.current.play();
    }
  };

  const onTime = () => {
    if (!ref.current || !timed) return;
    const t = ref.current.currentTime;
    let id: string | null = null;
    for (const s of slides) if (t >= s.startSec) id = s.id;
    onSlide(id);
  };

  if (!current) {
    return (
      <div className="frame">
        <div className="placeholder">Chưa có video. Duyệt script rồi bấm Render.</div>
      </div>
    );
  }

  return (
    <>
      <div className="frame">
        <video ref={ref} key={current.url} src={current.url} poster={poster} preload="metadata" controls loop={loop} onTimeUpdate={onTime}/>
      </div>
      <div className="player-controls">
        {renders.length > 1 && (
          <select value={renderId ?? ''} onChange={e => setRenderId(Number(e.target.value))} aria-label="Chọn bản render">
            {renders.map(r => <option key={r.id} value={r.id}>Bản v{r.version} · {r.startedAt}</option>)}
          </select>
        )}
        <select value={rate} onChange={e => setRate(Number(e.target.value))} aria-label="Tốc độ phát">
          {[0.75, 1, 1.25, 1.5].map(r => <option key={r} value={r}>{r}×</option>)}
        </select>
        <label className="small dim"><input type="checkbox" checked={loop} onChange={e => setLoop(e.target.checked)}/> Phát lặp</label>
        {current.version !== currentVersion && currentVersion > 0 && (
          <span className="small dim">Bản này dựng từ script v{current.version}; script hiện tại là v{currentVersion}.</span>
        )}
      </div>
    </>
  );
}
