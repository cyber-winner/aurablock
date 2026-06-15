// AuraBlock Frontend Controller

const API_BASE = window.location.origin;

// State management
let currentSection = 'overview';
let activeUpstreams = [];
let dbSize = '0 MB';
let statsInterval = null;
let eventSource = null;
let trafficChart = null;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  setupNavigation();
  initDashboard();
  setupForms();
  startLiveLogsStream();
});

// Sidebar Navigation
function setupNavigation() {
  const menuItems = document.querySelectorAll('.menu-item');
  menuItems.forEach(item => {
    item.addEventListener('click', (e) => {
      e.preventDefault();
      
      // Remove active class from all
      menuItems.forEach(m => m.classList.remove('active'));
      
      // Add active to clicked
      item.classList.add('active');
      
      // Hide all sections
      const targetSectionId = item.getAttribute('data-target');
      document.querySelectorAll('.content-section').forEach(sec => {
        sec.classList.remove('active');
      });
      
      // Show active section
      document.getElementById(`${targetSectionId}-section`).classList.add('active');
      
      // Update Title
      document.getElementById('current-section-title').textContent = item.querySelector('span').textContent;
      
      currentSection = targetSectionId;
      onSectionChanged(targetSectionId);
    });
  });
}

function onSectionChanged(sectionId) {
  if (sectionId === 'overview') {
    fetchStats();
    // Start polling stats every 5 seconds
    if (!statsInterval) {
      statsInterval = setInterval(fetchStats, 5000);
    }
  } else {
    // Stop polling stats when not on overview to save server resources
    if (statsInterval) {
      clearInterval(statsInterval);
      statsInterval = null;
    }
    
    if (sectionId === 'blocklists') {
      fetchBlocklists();
    } else if (sectionId === 'custom-rules') {
      fetchCustomRules();
    } else if (sectionId === 'blocked-history') {
      fetchBlockedHistory();
    } else if (sectionId === 'settings') {
      fetchSystemStatus();
    }
  }
}

// Initialize Dashboard
function initDashboard() {
  // Fetch initial system status
  fetchSystemStatus();
  
  // Load initial stats
  fetchStats();
  statsInterval = setInterval(fetchStats, 5000);
  
  // Global Toggle handler
  document.getElementById('global-toggle').addEventListener('click', toggleGlobalBlocking);
  
  // Clear logs handler
  document.getElementById('clear-logs').addEventListener('click', () => {
    document.getElementById('logs-stream-body').innerHTML = '';
  });
  
  // Force update blocklists
  document.getElementById('update-blocklists-btn').addEventListener('click', () => {
    fetch(`${API_BASE}/api/update`, { method: 'POST' })
      .then(res => res.json())
      .then(data => {
        alert('Blocklist update process started in background.');
        fetchStats();
      });
  });

  // Refresh blocked history logs
  document.getElementById('refresh-blocked').addEventListener('click', fetchBlockedHistory);
  document.getElementById('blocked-search').addEventListener('input', fetchBlockedHistory);
}

// Fetch System Status
function fetchSystemStatus() {
  fetch(`${API_BASE}/api/status`)
    .then(res => res.json())
    .then(status => {
      // Update UI Header
      activeUpstreams = status.upstreams || [];
      document.getElementById('topbar-upstream').textContent = activeUpstreams.map(u => u.split(':')[0]).join(', ');
      
      // Global Power State
      const indicator = document.getElementById('status-indicator');
      const text = document.getElementById('status-text');
      const toggleBtn = document.getElementById('global-toggle');
      
      if (status.blocking_enabled) {
        indicator.classList.add('active');
        text.textContent = 'Active';
        toggleBtn.className = 'btn btn-toggle';
        toggleBtn.innerHTML = '<i class="fa-solid fa-power-off"></i> Disable';
      } else {
        indicator.classList.remove('active');
        text.textContent = 'Disabled';
        toggleBtn.className = 'btn btn-primary';
        toggleBtn.innerHTML = '<i class="fa-solid fa-power-off"></i> Enable';
      }
      
      // Populate settings form if open
      const priInput = document.getElementById('primary-upstream');
      const secInput = document.getElementById('secondary-upstream');
      if (priInput && secInput && activeUpstreams.length >= 2) {
        priInput.value = activeUpstreams[0];
        secInput.value = activeUpstreams[1];
      } else if (priInput && activeUpstreams.length > 0) {
        priInput.value = activeUpstreams[0];
      }
    })
    .catch(err => console.error('Error fetching status:', err));
}

// Toggle Global Blocking
function toggleGlobalBlocking() {
  const indicator = document.getElementById('status-indicator');
  const isCurrentlyEnabled = indicator.classList.contains('active');
  
  fetch(`${API_BASE}/api/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled: !isCurrentlyEnabled })
  })
  .then(res => res.json())
  .then(data => {
    fetchSystemStatus();
  })
  .catch(err => console.error('Error toggling blocking:', err));
}

// Fetch Statistics and Render Charts
function fetchStats() {
  fetch(`${API_BASE}/api/stats`)
    .then(res => res.json())
    .then(stats => {
      // Update stats cards
      document.getElementById('stat-total').textContent = stats.total_queries.toLocaleString();
      document.getElementById('stat-blocked').textContent = stats.blocked_queries.toLocaleString();
      
      const percent = stats.percent_blocked.toFixed(1);
      document.getElementById('stat-percent').textContent = `${percent}%`;
      document.getElementById('stat-percent-progress').style.width = `${percent}%`;
      
      document.getElementById('stat-blocked-trend').textContent = `${percent}% block rate`;
      document.getElementById('stat-domains-count').textContent = stats.blocklist_domains.toLocaleString();
      
      // Update topbar db size
      const dbMB = (stats.db_size_bytes / (1024 * 1024)).toFixed(2);
      document.getElementById('topbar-dbsize').textContent = `${dbMB} MB`;

      // Render chart
      renderTrafficChart(stats.hourly_data || []);
      
      // Top lists
      populateDomainList('top-blocked-list', stats.top_blocked || [], true);
      populateDomainList('top-queried-list', stats.top_queried || [], false);
    })
    .catch(err => console.error('Error fetching stats:', err));
}

// Render traffic line chart using Chart.js
function renderTrafficChart(hourlyData) {
  const labels = [];
  const totals = [];
  const blocked = [];
  
  // Format hourly details (sort by hour)
  hourlyData.forEach(h => {
    labels.push(`${h.hour}:00`);
    totals.push(h.total);
    blocked.push(h.blocked);
  });
  
  if (labels.length === 0) {
    // Placeholder 24h labels if database empty
    for (let i = 0; i < 24; i++) {
      labels.push(`${String(i).padStart(2, '0')}:00`);
      totals.push(0);
      blocked.push(0);
    }
  }

  const ctx = document.getElementById('trafficChart').getContext('2d');
  
  if (trafficChart) {
    trafficChart.data.labels = labels;
    trafficChart.data.datasets[0].data = totals;
    trafficChart.data.datasets[1].data = blocked;
    trafficChart.update();
  } else {
    trafficChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [
          {
            label: 'Total Queries',
            data: totals,
            borderColor: '#9d4edd',
            backgroundColor: 'rgba(157, 78, 221, 0.06)',
            borderWidth: 2,
            pointBackgroundColor: '#9d4edd',
            pointRadius: 3,
            fill: true,
            tension: 0.4
          },
          {
            label: 'Blocked Ads',
            data: blocked,
            borderColor: '#ff5252',
            backgroundColor: 'rgba(255, 82, 82, 0.06)',
            borderWidth: 2,
            pointBackgroundColor: '#ff5252',
            pointRadius: 3,
            fill: true,
            tension: 0.4
          }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'top',
            labels: {
              color: '#8a99ad',
              font: { family: 'Outfit', size: 12 }
            }
          }
        },
        scales: {
          x: {
            grid: { color: 'rgba(255, 255, 255, 0.03)' },
            ticks: { color: '#8a99ad', font: { family: 'Outfit' } }
          },
          y: {
            grid: { color: 'rgba(255, 255, 255, 0.03)' },
            ticks: { color: '#8a99ad', font: { family: 'Outfit' }, precision: 0 }
          }
        }
      }
    });
  }
}

// Populate Domain Lists (Top Queried/Blocked)
function populateDomainList(listId, items, isBlockedList) {
  const list = document.getElementById(listId);
  list.innerHTML = '';
  
  if (items.length === 0) {
    list.innerHTML = '<li class="empty-state">No activity logged yet</li>';
    return;
  }
  
  items.forEach(item => {
    const li = document.createElement('li');
    li.innerHTML = `
      <span class="domain-name" title="${item.domain}">${item.domain}</span>
      <span class="domain-count" style="${isBlockedList ? 'color:#ff5252; background:rgba(255,82,82,0.08); border-color:rgba(255,82,82,0.15)' : ''}">${item.count}</span>
    `;
    list.appendChild(li);
  });
}

// Live Logs Stream (SSE)
function startLiveLogsStream() {
  if (eventSource) {
    eventSource.close();
  }
  
  eventSource = new EventSource(`${API_BASE}/api/logs/stream`);
  
  eventSource.onmessage = (event) => {
    const log = JSON.parse(event.data);
    appendLogToTable(log);
  };
  
  eventSource.onerror = (err) => {
    console.error('SSE Error, reconnecting in 5s...', err);
    eventSource.close();
    setTimeout(startLiveLogsStream, 5000);
  };
}

function appendLogToTable(log) {
  const tbody = document.getElementById('logs-stream-body');
  
  // Query Filter check
  const searchVal = document.getElementById('log-search').value.toLowerCase();
  if (searchVal && !log.domain.includes(searchVal)) {
    return;
  }
  
  const tr = document.createElement('tr');
  
  // Status Badge Class
  let badgeClass = 'badge-allow';
  let statusText = 'Allowed';
  if (log.status === 'BLOCKED') {
    badgeClass = 'badge-block';
    statusText = 'Blocked';
  } else if (log.status.includes('CACHE')) {
    badgeClass = 'badge-allow-cache';
    statusText = 'Cached';
  } else if (log.status === 'ERROR') {
    badgeClass = 'badge-error';
    statusText = 'Error';
  }
  
  const timeStr = new Date(log.timestamp).toLocaleTimeString();
  
  tr.innerHTML = `
    <td class="col-time">${timeStr}</td>
    <td class="col-domain" title="${log.domain}">${log.domain}</td>
    <td><span class="col-type">${log.query_type}</span></td>
    <td class="col-client">${log.client_ip}</td>
    <td><span class="badge ${badgeClass}">${statusText}</span></td>
    <td class="col-ans" title="${log.answer}">${log.answer || '—'}</td>
    <td class="col-latency">${log.response_time_ms} ms</td>
  `;
  
  // Insert at top
  tbody.insertBefore(tr, tbody.firstChild);
  
  // Truncate to keep performance
  if (tbody.children.length > 150) {
    tbody.removeChild(tbody.lastChild);
  }
}

// Fetch Blocklists
function fetchBlocklists() {
  fetch(`${API_BASE}/api/lists`)
    .then(res => res.json())
    .then(lists => {
      const container = document.getElementById('blocklists-manager-container');
      container.innerHTML = '';
      
      if (lists.length === 0) {
        container.innerHTML = '<div class="empty-state">No blocklists configured</div>';
        return;
      }
      
      lists.forEach(list => {
        const div = document.createElement('div');
        div.className = 'list-manager-item';
        
        const dateStr = list.last_updated && !list.last_updated.startsWith('0001')
          ? new Date(list.last_updated).toLocaleString()
          : 'Never updated';
          
        div.innerHTML = `
          <div class="list-meta">
            <h4>${list.name}</h4>
            <p title="${list.url}">${list.url}</p>
            <div class="details">
              <span><i class="fa-solid fa-database"></i> ${list.item_count.toLocaleString()} domains</span>
              <span><i class="fa-solid fa-clock"></i> Updated: ${dateStr}</span>
            </div>
          </div>
          <div class="list-controls">
            <label class="switch">
              <input type="checkbox" ${list.enabled ? 'checked' : ''} onchange="toggleList(${list.id}, this.checked)">
              <span class="slider"></span>
            </label>
            <button class="btn-danger-small" onclick="deleteList(${list.id})"><i class="fa-solid fa-trash"></i></button>
          </div>
        `;
        container.appendChild(div);
      });
    })
    .catch(err => console.error('Error fetching blocklists:', err));
}

// Toggle List state
function toggleList(id, enabled) {
  fetch(`${API_BASE}/api/lists/${id}/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled })
  })
  .then(res => res.json())
  .then(() => fetchBlocklists())
  .catch(err => console.error('Error toggling list:', err));
}

// Delete List
function deleteList(id) {
  if (confirm('Are you sure you want to delete this blocklist?')) {
    fetch(`${API_BASE}/api/lists/${id}`, { method: 'DELETE' })
      .then(res => res.json())
      .then(() => fetchBlocklists())
      .catch(err => console.error('Error deleting list:', err));
  }
}

// Fetch Custom Whitelist/Blacklist Rules
function fetchCustomRules() {
  fetch(`${API_BASE}/api/rules`)
    .then(res => res.json())
    .then(rules => {
      const tbody = document.getElementById('rules-list-body');
      tbody.innerHTML = '';
      
      const filter = document.getElementById('rule-search').value.toLowerCase();
      
      const filtered = rules.filter(r => r.domain.toLowerCase().includes(filter));
      
      if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="empty-state">No custom exception rules configured</td></tr>';
        return;
      }
      
      filtered.forEach(rule => {
        const tr = document.createElement('tr');
        const badgeClass = rule.rule_type === 'ALLOW' ? 'badge-allow' : 'badge-block';
        const typeText = rule.rule_type === 'ALLOW' ? 'Whitelist' : 'Blacklist';
        
        tr.innerHTML = `
          <td style="font-family:'JetBrains Mono', monospace; font-weight:500; color:#fff;">${rule.domain}</td>
          <td><span class="badge ${badgeClass}">${typeText}</span></td>
          <td style="color:var(--text-muted);">${rule.comment || '—'}</td>
          <td><button class="btn-danger-small" onclick="deleteRule(${rule.id})"><i class="fa-solid fa-trash"></i> Delete</button></td>
        `;
        tbody.appendChild(tr);
      });
    })
    .catch(err => console.error('Error fetching custom rules:', err));
}

// Delete Custom Rule
function deleteRule(id) {
  fetch(`${API_BASE}/api/rules/${id}`, { method: 'DELETE' })
    .then(res => res.json())
    .then(() => fetchCustomRules())
    .catch(err => console.error('Error deleting custom rule:', err));
}

// Setup Form Handlers
function setupForms() {
  // Add Blocklist Form
  document.getElementById('add-list-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const name = document.getElementById('list-name').value;
    const url = document.getElementById('list-url').value;
    
    fetch(`${API_BASE}/api/lists`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, url })
    })
    .then(res => res.json())
    .then(() => {
      document.getElementById('add-list-form').reset();
      fetchBlocklists();
      alert('List added! AuraBlock will update domains in background.');
    })
    .catch(err => console.error('Error adding list:', err));
  });

  // Add Custom Exception Rule Form
  document.getElementById('add-rule-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const domain = document.getElementById('rule-domain').value;
    const rule_type = document.querySelector('input[name="rule-type"]:checked').value;
    const comment = document.getElementById('rule-comment').value;
    
    fetch(`${API_BASE}/api/rules`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain, rule_type, comment })
    })
    .then(res => res.json())
    .then(() => {
      document.getElementById('add-rule-form').reset();
      fetchCustomRules();
    })
    .catch(err => console.error('Error adding exception rule:', err));
  });

  // Upstream DNS Save Form
  document.getElementById('upstream-settings-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const pri = document.getElementById('primary-upstream').value;
    const sec = document.getElementById('secondary-upstream').value;
    
    fetch(`${API_BASE}/api/upstreams`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ upstreams: [pri, sec] })
    })
    .then(res => res.json())
    .then(() => {
      fetchSystemStatus();
      alert('Upstream DNS servers updated successfully!');
    })
    .catch(err => console.error('Error saving upstreams:', err));
  });
  
  // Custom rules search filter
  document.getElementById('rule-search').addEventListener('input', fetchCustomRules);
}

// Fetch and render blocked query history
function fetchBlockedHistory() {
  fetch(`${API_BASE}/api/history?status=BLOCKED&limit=200`)
    .then(res => res.json())
    .then(logs => {
      const tbody = document.getElementById('blocked-history-body');
      tbody.innerHTML = '';
      
      const filter = document.getElementById('blocked-search').value.toLowerCase();
      const filtered = logs.filter(l => l.domain.toLowerCase().includes(filter));
      
      if (filtered.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No blocked queries found</td></tr>';
        return;
      }
      
      filtered.forEach(log => {
        const tr = document.createElement('tr');
        const timeStr = new Date(log.timestamp).toLocaleString();
        
        tr.innerHTML = `
          <td class="col-time">${timeStr}</td>
          <td class="col-domain" style="color:#ff5252; font-family:'JetBrains Mono', monospace;" title="${log.domain}">${log.domain}</td>
          <td><span class="col-type">${log.query_type}</span></td>
          <td class="col-client">${log.client_ip}</td>
          <td class="col-ans">${log.answer || '0.0.0.0'}</td>
        `;
        tbody.appendChild(tr);
      });
    })
    .catch(err => console.error('Error fetching blocked history:', err));
}
