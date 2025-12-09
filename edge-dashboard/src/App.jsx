import { useState, useEffect, useRef } from 'react';
import './App.css';

// Configure WebSocket Port (If you run Node Go port 4001, WS will be 8001)
const WS_URL = 'ws://localhost:8001/ws';

function App() {
  const [isConnected, setIsConnected] = useState(false);
  const [logs, setLogs] = useState([]);
  const [nodes, setNodes] = useState({}); // Save list of Workers met
  const socketRef = useRef(null);
  const logsEndRef = useRef(null);

  // Handle events from Go sent
  const handleNewEvent = (data) => {
    // Debug: Open browser console (F12) to see what data is received
    // console.log("Received WebSocket Data:", data); 

    // CASE 1: Task completed message -> Save log + Update Node
    if (data.type === 'TASK_COMPLETED') {
      const timestamp = new Date().toLocaleTimeString();
      
      // ONLY setLogs WHEN IT'S TASK_COMPLETED
      setLogs(prev => [...prev, { ...data, time: timestamp }]);

      // Animation for Node
      const workerID = data.worker;
      setNodes(prev => ({
        ...prev,
        [workerID]: { ...prev[workerID], lastSeen: Date.now(), active: true }
      }));

      setTimeout(() => {
        setNodes(prev => ({
          ...prev,
          [workerID]: { ...prev[workerID], active: false }
        }));
      }, 1000);
    }

    // CASE 2: Heartbeat message -> ONLY Update Node (NO LOG)
    else if (data.type === 'HEARTBEAT') {
      setNodes(prev => ({
        ...prev,
        [data.node_id]: { 
          ...prev[data.node_id], 
          lastSeen: Date.now(),
          cpu: data.cpu,
          ram: data.ram
        }
      }));
    }
  };

  useEffect(() => {
    // 1. Initialize WebSocket Connection
    const connect = () => {
      const ws = new WebSocket(WS_URL);
      socketRef.current = ws;

      ws.onopen = () => {
        console.log('Connected to Edge Grid Node');
        setIsConnected(true);
      };

      ws.onclose = () => {
        console.log('Disconnected');
        setIsConnected(false);
        // Automatically reconnect after 3s if connection is lost
        setTimeout(connect, 3000);
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          handleNewEvent(data);
        } catch (event) {
          console.error("Invalid JSON:", event.data);
        }
      };
    };

    connect();

    return () => {
      if (socketRef.current) socketRef.current.close();
    };
  }, []);

  // Automatically scroll to bottom of terminal when new log is added
  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  return (
    <div className="dashboard-container">
      <header>
        <h1>NEBULA-MESH // GOD MODE</h1>
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <span className={`status-indicator ${isConnected ? 'connected' : ''}`}></span>
          {isConnected ? 'SYSTEM ONLINE' : 'DISCONNECTED'}
        </div>
      </header>

      {/* Left column: Terminal Logs */}
      <div className="terminal-window">
        <div className="terminal-header">
          root@gateway:~# tail -f /var/log/edge-grid.log
        </div>
        <div className="terminal-content">
          {logs.length === 0 && <div style={{color: '#444'}}>Waiting for tasks...</div>}
          
          {logs.map((log, index) => (
            <div key={index} className="log-entry">
              <span className="log-time">[{log.time}]</span>
              <span>Task </span>
              <span style={{color: '#fff'}}>{log.task_id?.split('-')[1]}</span>
              <span> completed by </span>
              <span className="log-worker">{log.worker}</span>
              <br/>
              <span>&gt;&gt; Output: </span>
              <span className="log-result">{log.result}</span>
            </div>
          ))}
          <div ref={logsEndRef} />
        </div>
      </div>

      {/* Right column: Active Nodes Visualizer */}
      <div className="nodes-panel">
        <h3>ACTIVE WORKERS</h3>
        <div className="node-grid">
          {Object.keys(nodes).map(nodeId => {
            const node = nodes[nodeId];
            // Calculate color based on CPU load (Green -> Red)
            const cpuColor = node.cpu > 80 ? 'red' : node.cpu > 50 ? 'orange' : '#00ff41';
            
            return (
              <div 
                key={nodeId} 
                className={`node-card ${node.active ? 'active' : ''}`}
                style={{textAlign: 'left'}}
              >
                <div style={{display: 'flex', justifyContent: 'space-between', marginBottom: '5px'}}>
                  <span>🤖 {nodeId}</span>
                  <span style={{fontSize: '0.7rem', color: '#888'}}>{node.ram}MB</span>
                </div>
                
                {/* CPU Bar */}
                <div style={{fontSize: '0.7rem', marginBottom: '2px'}}>CPU: {node.cpu?.toFixed(1)}%</div>
                <div style={{
                  width: '100%', 
                  height: '4px', 
                  background: '#333', 
                  borderRadius: '2px'
                }}>
                  <div style={{
                    width: `${node.cpu}%`, 
                    height: '100%', 
                    background: cpuColor,
                    transition: 'width 0.5s ease'
                  }}></div>
                </div>
              </div>
            )
          })}
          
          {Object.keys(nodes).length === 0 && (
            <div style={{color: '#666', gridColumn: '1/-1'}}>No active workers detected yet.</div>
          )}
        </div>
        
        <div style={{marginTop: '20px', fontSize: '0.8rem', color: '#666'}}>
          <p>Topology: P2P Mesh</p>
          <p>Protocol: QUIC/UDP</p>
          <p>Runtime: Wazero (Wasm)</p>
        </div>
      </div>
    </div>
  );
}

export default App;