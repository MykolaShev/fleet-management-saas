import { useEffect, useState } from 'react';
import { api } from '../api';
import StatusBadge from '../StatusBadge';

export default function Vehicles() {
  const [vehicles, setVehicles] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    api('/vehicles?page_size=200')
      .then((data) => setVehicles(data.items || []))
      .catch((err) => setError(err.message));
  }, []);

  if (error) return <p className="error">{error}</p>;
  if (!vehicles) return <p className="muted">Loading…</p>;

  if (vehicles.length === 0) {
    return (
      <div>
        <h1>Vehicles</h1>
        <p className="empty-state">
          No vehicles yet. Create one with <code>POST /vehicles</code> — it'll show up here.
        </p>
      </div>
    );
  }

  return (
    <div>
      <h1>Vehicles <span className="count">{vehicles.length}</span></h1>
      <table>
        <thead>
          <tr>
            <th>Plate</th>
            <th>Model</th>
            <th>Status</th>
            <th>Position</th>
          </tr>
        </thead>
        <tbody>
          {vehicles.map((v) => (
            <tr key={v.id}>
              <td className="mono">{v.plate_number}</td>
              <td>{v.model}</td>
              <td><StatusBadge status={v.status} /></td>
              <td className="mono muted">
                {v.current_lat != null
                  ? `${v.current_lat.toFixed(4)}, ${v.current_lng.toFixed(4)}`
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}