import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';

// Dev-token sign-in, same shortcut used for the Postman demos — mints a
// real session JWT against a fresh (or named) tenant without a real
// Google login. F4 adds an actual "Sign in with Microsoft" button
// alongside this, not instead of it.
export default function Login() {
  const [role, setRole] = useState('manager');
  const [error, setError] = useState(null);
  const navigate = useNavigate();

  async function signIn(e) {
    e.preventDefault();
    setError(null);
    try {
      const body = await api('/auth/dev-token', {
        method: 'POST',
        body: JSON.stringify({ role }),
      });
      localStorage.setItem('token', body.token);
      localStorage.setItem('tenant_id', body.tenant_id);
      navigate('/vehicles');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="centered">
      <form onSubmit={signIn} className="card">
        <h1>Fleet Management</h1>
        <p className="muted">Dev sign-in — mints a session for a fresh tenant.</p>
        <label>
          Role
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="manager">manager</option>
            <option value="driver">driver</option>
          </select>
        </label>
        <button type="submit">Sign in</button>
        {error && <p className="error">{error}</p>}
      </form>
    </div>
  );
}