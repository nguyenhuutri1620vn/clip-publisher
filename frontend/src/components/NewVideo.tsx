import {useEffect, useState} from 'react';
import {api, errText} from '../api';

const sample = `CẢNH 1
Thời gian: 0:00 – 0:04
Hình ảnh: Nền đất tối, một mũi tên đồng ba cạnh nằm giữa khung
Chữ trên hình: TRUYỀN THUYẾT NỎ THẦN… CÓ BẰNG CHỨNG DƯỚI LÒNG ĐẤT
Lời đọc: Truyền thuyết nỏ thần mà ai cũng nghĩ là chuyện cổ tích… hóa ra có bằng chứng nằm ngay dưới lòng đất.

CẢNH 2
…`;

export default function NewVideo({onCreated}: {onCreated: (id: number) => void}) {
  const [text, setText] = useState('');
  const [voices, setVoices] = useState<string[]>([]);
  const [music, setMusic] = useState<string[]>([]);
  const [voice, setVoice] = useState('');
  const [track, setTrack] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  useEffect(() => {
    api.GetSettings().then(s => { setVoice(s.default_voice); setTrack(s.default_music); });
    api.ListVoices().then(v => setVoices(v ?? [])).catch(() => {});
    api.ListMusic().then(m => setMusic(m ?? [])).catch(() => {});
  }, []);

  const create = () => {
    setBusy(true);
    setErr('');
    api.CreateVideo(text, voice, track).then(onCreated).catch(e => { setErr(errText(e)); setBusy(false); });
  };

  return (
    <div className="page">
      <h2>Video mới</h2>
      <p className="dim">
        Dán kịch bản theo từng cảnh: Thời gian, Hình ảnh, Chữ trên hình, Lời đọc. Một ý tưởng ngắn cũng được.
        Claude sẽ viết script, bạn duyệt rồi mới render.
      </p>
      <div className="new-form">
        <textarea value={text} onChange={e => setText(e.target.value)} placeholder={sample} autoFocus/>
        <div className="field-row">
          <label className="field">
            Giọng đọc
            <select value={voice} onChange={e => setVoice(e.target.value)}>
              {voices.map(v => <option key={v} value={v}>{v}</option>)}
            </select>
          </label>
          <label className="field">
            Nhạc nền
            <select value={track} onChange={e => setTrack(e.target.value)}>
              <option value="">Không có nhạc</option>
              {music.map(m => <option key={m} value={m}>{m}</option>)}
            </select>
          </label>
          <button className="primary" disabled={busy || !text.trim()} onClick={create}>
            {busy ? 'Đang gửi cho Claude…' : 'Tạo script'}
          </button>
        </div>
        {err && <div className="banner error">{err}</div>}
      </div>
    </div>
  );
}
