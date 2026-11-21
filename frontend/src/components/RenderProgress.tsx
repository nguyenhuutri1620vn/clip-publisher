import {useEffect, useState} from 'react';
import {fmtDuration, ProgressDTO} from '../api';

const renderSteps = [
  {key: 'validate', label: 'Kiểm tra script'},
  {key: 'export', label: 'Xuất lời đọc'},
  {key: 'voice', label: 'Đọc voice'},
  {key: 'audio', label: 'Ghép audio'},
  {key: 'render', label: 'Render video'},
];

function useTicker(base: number) {
  const [start] = useState(() => Date.now() - base * 1000);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  return (now - start) / 1000;
}

export default function RenderProgress({kind, progress, log}: {kind: 'claude' | 'render'; progress: ProgressDTO | null; log: string[]}) {
  const elapsed = useTicker(progress?.elapsedSec ?? 0);

  if (kind === 'claude') {
    return (
      <div className="progress-card">
        <div className="serif" style={{fontSize: 18}}>Claude đang viết script…</div>
        <div className="bigtime">{fmtDuration(elapsed) || '0:00'}</div>
        <p className="dim small" style={{margin: 0}}>
          Thường mất 1–3 phút. Script sai cấu trúc sẽ được gửi lại cho Claude sửa, tối đa 3 lần.
        </p>
      </div>
    );
  }

  const queued = progress && progress.queuePos > 0;
  const idx = renderSteps.findIndex(s => s.key === progress?.step);
  const left = progress ? Math.max(0, (progress.etaSec ?? 0) - (elapsed - (progress.elapsedSec ?? 0))) : 0;

  return (
    <div className="progress-card">
      {queued ? (
        <div className="serif" style={{fontSize: 18}}>Đang chờ video khác render xong (vị trí #{progress!.queuePos})</div>
      ) : (
        <>
          <div style={{display: 'flex', alignItems: 'baseline', gap: 14}}>
            <div className="bigtime">{fmtDuration(elapsed) || '0:00'}</div>
            {left > 0 && <span className="dim">còn khoảng {fmtDuration(left)}</span>}
          </div>
          <ol className="steps">
            {renderSteps.map((s, i) => {
              const state = i < idx ? 'done' : i === idx ? 'now' : '';
              let label = s.label;
              if (s.key === 'voice' && progress) {
                label += progress.voiceTotal > 0
                  ? ` ${Math.min(progress.voiceDone, progress.voiceTotal)}/${progress.voiceTotal}`
                  : ' (đã có sẵn)';
                if (state === 'now' && progress.voiceId) label += ` · cảnh ${progress.voiceId}`;
              }
              return (
                <li key={s.key} className={state}>
                  <span className="dot">{state === 'done' ? '✓' : i + 1}</span>
                  <span>{label}</span>
                </li>
              );
            })}
          </ol>
        </>
      )}
      {log.length > 0 && (
        <details>
          <summary className="dim small">Log chi tiết</summary>
          <div className="log">{log.join('\n')}</div>
        </details>
      )}
    </div>
  );
}
