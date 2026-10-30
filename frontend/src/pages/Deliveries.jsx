import { useEffect, useState } from 'react';
import { api } from '../api';

export default function Deliveries() {
  const [deliveries, setDeliveries] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    api('/deliveries?page_size=200')
      .then((data) => setDeliveries(data.items || []))
      .catch((err) => setError(err.message));
  }, []);

  if (error) return <p className="error">{error}</p>;
  if (!deliveries) return <p>Loading…</p>;

  return (
    <div>
      <h2>Deliveries ({deliveries.length})</h2>
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
              <td>{d.status}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}