import {useEffect, useRef, useState} from 'react';
import {api, VideoDetail} from '../api';

type Props = {d: VideoDetail; busy: boolean; act: (p: Promise<unknown>) => Promise<unknown>};

export default function ChatPanel({d, busy, act}: Props) {
  const [text, setText] = useState('');
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => { end.current?.scrollIntoView({block: 'end'}); }, [d.chat.length]);

  const send = () => {
    const msg = text.trim();
    if (!msg) return;
    act(api.Chat(d.video.id, msg)).then(() => setText(''));
  };

  return (
    <>
      <div className="tab-body">
        {d.chat.length === 0 && (
          <p className="dim">
            Nói với Claude điều muốn đổi, ví dụ “cảnh 2 ngắn lại”, “hook gắt hơn”, “thêm một cảnh về khuôn đúc”.
            Mỗi lần sửa thành một phiên bản script mới; bạn duyệt rồi render lại.
          </p>
        )}
        <div className="chat">
          {d.chat.map((m, i) => <div key={i} className={`msg ${m.role}`}>{m.text}</div>)}
          {d.video.status === 'GENERATING_SCRIPT' && <div className="msg assistant dim">Claude đang sửa script…</div>}
          <div ref={end}/>
        </div>
      </div>
      <div className="chat-input">
        <textarea value={text} onChange={e => setText(e.target.value)} disabled={busy}
                  placeholder={busy ? 'Đợi việc đang chạy xong…' : 'Yêu cầu sửa script'}
                  onKeyDown={e => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) send(); }}/>
        <button className="primary" disabled={busy || !text.trim()} onClick={send} title="⌘ Enter">Gửi</button>
      </div>
    </>
  );
}
