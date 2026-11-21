import {statusLabel} from '../api';

export default function StatusBadge({status, queuePos}: {status: string; queuePos?: number}) {
  const waiting = status === 'RENDERING' && queuePos !== undefined && queuePos > 0;
  return (
    <span className={`badge ${status}`}>
      {waiting ? `Chờ render #${queuePos}` : statusLabel[status] ?? status}
    </span>
  );
}
