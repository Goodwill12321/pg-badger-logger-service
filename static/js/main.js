let currentServer = '';
let currentReport = '';
let statusCheckInterval = null;

document.addEventListener('DOMContentLoaded', () => {
    // Server selection
    document.querySelectorAll('#serverList .list-group-item').forEach(item => {
        item.addEventListener('click', (e) => {
            e.preventDefault();
            const serverName = e.target.dataset.server;
            selectServer(serverName);
        });
    });

    // Refresh button
    document.getElementById('refreshBtn').addEventListener('click', () => {
        if (currentServer) {
            loadServerData(currentServer);
        }
    });

    // Confirm report generation
    document.getElementById('confirmGenerate').addEventListener('click', () => {
        const modal = bootstrap.Modal.getInstance(document.getElementById('confirmModal'));
        modal.hide();
        generateReport(currentServer, currentReport);
    });

    // Stop report generation
    document.getElementById('stopGeneration').addEventListener('click', () => {
        if (currentServer && currentReport) {
            stopReportGeneration(currentServer, currentReport);
        }
    });
});

function selectServer(serverName) {
    currentServer = serverName;
    document.getElementById('serverTitle').textContent = serverName;
    document.getElementById('refreshBtn').style.display = 'inline-block';
    loadServerData(serverName);
}

function loadServerData(serverName) {
    Promise.all([
        fetch(`/api/logs/${serverName}`).then(res => res.json()),
        fetch(`/api/reports/${serverName}`).then(res => res.json())
    ]).then(([logs, reports]) => {
        displayLogs(logs);
        displayReports(reports);
    }).catch(error => {
        console.error('Error loading server data:', error);
        alert('Failed to load server data');
    });
}

function displayLogs(logs) {
    const logsDiv = document.getElementById('logsList');
    const logsContent = document.getElementById('logsListContent');
    logsDiv.style.display = 'block';
    
    logsContent.innerHTML = logs.map(log => `
        <a href="#" class="list-group-item list-group-item-action" onclick="confirmGenerateReport('${log.name}')">
            <div class="d-flex w-100 justify-content-between">
                <h6 class="mb-1">${log.name}</h6>
                <small>${formatSize(log.size)}</small>
            </div>
            <small class="text-muted">${new Date(log.date).toLocaleString()}</small>
        </a>
    `).join('');
}

function displayReports(reports) {
    const reportsDiv = document.getElementById('reportsList');
    const reportsContent = document.getElementById('reportsListContent');
    reportsDiv.style.display = 'block';
    
    reportsContent.innerHTML = reports.map(report => `
        <div class="list-group-item">
            <div class="report-item">
                <div class="report-info">
                    <h6 class="mb-1">${report.name}</h6>
                    <small class="text-muted">Created: ${new Date(report.createdAt).toLocaleString()}</small>
                    ${report.isProcessing ? '<span class="badge bg-warning processing-badge">Processing</span>' : ''}
                </div>
                <div class="report-actions">
                    ${report.isProcessing ?
                        `<button class="btn btn-sm btn-info" onclick="checkReportStatus('${report.name}')">View Status</button>` :
                        `<a href="/report/${currentServer}/${report.name}" target="_blank" class="btn btn-sm btn-primary">View Report</a>`
                    }
                </div>
            </div>
        </div>
    `).join('');
}

function confirmGenerateReport(logName) {
    currentReport = logName;
    const modal = new bootstrap.Modal(document.getElementById('confirmModal'));
    modal.show();
}

function generateReport(serverName, logName) {
    const formData = new FormData();
    formData.append('logFile', logName);

    fetch(`/api/report/${serverName}`, {
        method: 'POST',
        body: formData
    })
    .then(res => res.json())
    .then(data => {
        if (data.error) {
            alert(data.error);
        } else {
            checkReportStatus(data.report);
        }
    })
    .catch(error => {
        console.error('Error generating report:', error);
        alert('Failed to generate report');
    });
}

function checkReportStatus(reportName) {
    const modal = new bootstrap.Modal(document.getElementById('statusModal'));
    modal.show();
    
    function updateStatus() {
        fetch(`/api/report-status/${currentServer}/${reportName}`)
            .then(res => res.json())
            .then(data => {
                const statusOutput = document.getElementById('statusOutput');
                if (data.error) {
                    statusOutput.textContent = `Error: ${data.error}`;
                    return;
                }

                statusOutput.textContent = data.output || 'No output available';
                
                if (data.status === 'completed') {
                    clearInterval(statusCheckInterval);
                    loadServerData(currentServer);
                }
            })
            .catch(error => {
                console.error('Error checking status:', error);
                document.getElementById('statusOutput').textContent = 'Failed to check status';
            });
    }

    updateStatus();
    statusCheckInterval = setInterval(updateStatus, 2000);

    document.getElementById('statusModal').addEventListener('hidden.bs.modal', () => {
        clearInterval(statusCheckInterval);
    });
}

function stopReportGeneration(serverName, reportName) {
    fetch(`/api/stop-report/${serverName}/${reportName}`, {
        method: 'POST'
    })
    .then(res => res.json())
    .then(data => {
        if (data.error) {
            alert(data.error);
        } else {
            loadServerData(serverName);
            const modal = bootstrap.Modal.getInstance(document.getElementById('statusModal'));
            modal.hide();
        }
    })
    .catch(error => {
        console.error('Error stopping report generation:', error);
        alert('Failed to stop report generation');
    });
}

function formatSize(bytes) {
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let unitIndex = 0;
    
    while (size >= 1024 && unitIndex < units.length - 1) {
        size /= 1024;
        unitIndex++;
    }
    
    return `${size.toFixed(1)} ${units[unitIndex]}`;
}
