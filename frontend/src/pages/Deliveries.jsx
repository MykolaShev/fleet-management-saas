import { useEffect, useState } from 'react';
import { api } from '../api';
import StatusBadge from '../StatusBadge';

export default function Deliveries() {
  const [deliveries, setDeliveries] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    api('/deliveries?page_size=200')
      .then((data) => setDeliveries(data.items || []))
      .catch((err) => setError(err.message));
  }, []);

  if (error) return <p className="error">{error}</p>;
  if (!deliveries) return <p className="muted">Loading…</p>;

  if (deliveries.length === 0) {
    return (
      <div>
        <h1>Deliveries</h1>
        <p className="empty-state">
          No deliveries yet. Create one with <code>POST /deliveries</code> — it'll show up here.
        </p>
      </div>
    );
  }

  return (
    <div>
      <h1>Deliveries <span className="count">{deliveries.length}</span></h1>
      <table>
        <thead>
          <tr>
            <th>Pickup</th>
            <th>Dropoff</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {deliveries.map((d) => (
            <tr key={d.id}>
              <td>{d.pickup_address}</td>
              <td>{d.dropoff_address}</td>
              <td><StatusBadge status={d.status} /></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}