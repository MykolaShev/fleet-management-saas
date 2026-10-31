// Status color means something specific per domain, not a generic
// palette rotation: grey = parked/not moving, amber = active/in motion,
// red = needs attention, green = done.
const COLORS = {
    idle: '#9CA3AF',
    en_route: '#E8871E',
    maintenance: '#DC2626',
    pending: '#9CA3AF',
    assigned: '#E8871E',
    in_transit: '#E8871E',
    delivered: '#16A34A',
    failed: '#DC2626',
  };
  
  export default function StatusBadge({ status }) {
    return (
      <span className="status-badge">
        <span className="status-dot" style={{ background: COLORS[status] || '#9CA3AF' }} />
        {status}
      </span>
    );
  }