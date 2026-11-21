import {useEffect, useState} from 'react';
import {api, Check, errText, Settings as S} from '../api';
import {config} from '../../wailsjs/go/models';

const fields: {key: keyof S; label: string; hint?: string; readOnly?: boolean}[] = [
  {key: 'video_repo', label: 'Thư mục make-video-free'},
  {key: 'video_python', label: 'Python của make-video-free', hint: 'Thường là <thư mục>/.venv/bin/python'},
  {key: 'voice_repo', label: 'Thư mục make-voice-free'},
  {key: 'voice_bin', label: 'Lệnh make-voice-free', hint: 'Thường là <thư mục>/.venv/bin/make-voice-free'},
  {key: 'claude_bin', label: 'Lệnh Claude Code'},
  {key: 'workspace', label: 'Thư mục làm việc', readOnly: true, hint: 'Cố định trong bản này'},
];

export default function Settings() {
  const [s, setS] = useState<S | null>(null);
  const [voices, setVoices] = useState<string[]>([]);
  const [music, setMusic] = useState<string[]>([]);
  const [checks, setChecks] = useState<Check[] | null>(null);
  const [checking, setChecking] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  const loadLists = () => {
    api.ListVoices().then(v => setVoices(v ?? [])).catch(() => setVoices([]));
    api.ListMusic().then(m => setMusic(m ?? [])).catch(() => setMusic([]));
  };
  useEffect(() => { api.GetSettings().then(setS); loadLists(); }, []);

  if (!s) return <div className="page dim">Đang tải…</div>;

  const set = (k: keyof S, v: string) => { setS(config.Settings.createFrom({...s, [k]: v})); setMsg(''); };
  const save = () => api.SaveSettings(s)
    .then(() => { setMsg('Đã lưu'); setErr(''); loadLists(); })
    .catch(e => setErr(errText(e)));
  const doctor = () => {
    setChecking(true);
    api.RunDoctor().then(setChecks).finally(() => setChecking(false));
  };

  return (
    <div className="page">
      <div className="page-head"><h2>Cài đặt</h2></div>
      <div className="settings">
        <p className="dim" style={{margin: 0}}>
          App gọi 2 project có sẵn trên máy và Claude Code. Đường dẫn sai thì bấm “Kiểm tra công cụ” sẽ thấy ngay.
        </p>

        <div className="group">
          {fields.map(f => (
            <label key={f.key} className="group-row">
              <span>{f.label}</span>
              <input type="text" value={s[f.key] as string} readOnly={f.readOnly} disabled={f.readOnly}
                     onChange={e => set(f.key, e.target.value)}/>
              {f.hint && <span className="hint small dim">{f.hint}</span>}
            </label>
          ))}
        </div>

        <div className="group">
          <label className="group-row">
            <span>Giọng đọc mặc định</span>
            <select value={s.default_voice} onChange={e => set('default_voice', e.target.value)}>
              {voices.map(v => <option key={v} value={v}>{v}</option>)}
            </select>
          </label>
          <label className="group-row">
            <span>Nhạc nền mặc định</span>
            <select value={s.default_music} onChange={e => set('default_music', e.target.value)}>
              <option value="">Không có nhạc</option>
              {music.map(m => <option key={m} value={m}>{m}</option>)}
            </select>
          </label>
        </div>

        <div className="toolbar">
          <button onClick={doctor} disabled={checking}>{checking ? 'Đang kiểm tra…' : 'Kiểm tra công cụ'}</button>
          <span className="grow"/>
          {msg && <span className="ok small">{msg}</span>}
          <button className="primary" onClick={save}>Lưu</button>
        </div>
        {err && <div className="banner error">{err}</div>}
        {checks && (
          <ul className="checks group">
            {checks.map(c => (
              <li key={c.name}>
                <span className={c.ok ? 'ok' : 'bad'}>{c.ok ? '✓' : '✗'}</span>
                <span>{c.name}</span>
                <span className="dim small selectable">{c.detail}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
