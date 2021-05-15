package infrared

type testAddr struct {
	network string
	ip      string
}

func (addr testAddr) Network() string {
	return addr.network
}

func (addr testAddr) String() string {
	return addr.ip
}

// func TestHandleConn_OfflineServer(t *testing.T) {
// 	domain := serverDomain
// 	proxyTo := ":25560"

// 	c1, c2 := net.Pipe()
// 	cServer := createTestConn(c1)
// 	cClient := createTestConn(c2)

// 	connAddr := &testAddr{network: "tcp", ip: "127.0.0.1"}

// 	proxyConfig := &ProxyConfig{DomainName: domain, ProxyTo: proxyTo}
// 	proxy := &Proxy{Config: proxyConfig}

// 	fmt.Printf("timeout time: %v\n", proxy.Timeout())

// 	go func(c Conn) {
// 		pk := serverHandshake(domain, 25565)
// 		sendHandshake(c, pk)

// 		statusPk := status.ServerBoundRequest{}.Marshal()
// 		if err := c.WritePacket(statusPk); err != nil {
// 			return
// 		}

// 		c.ReadPacket()

// 		c.Close()

// 	}(cClient)

// 	if err := proxy.handleConn(cServer, connAddr); err != nil {
// 		fmt.Println("Error is not nil")
// 		fmt.Println(err)
// 		t.Fail()
// 		return
// 	}
// 	fmt.Println("Error is nil")
// }

// func TestHandleStatusRequest(t *testing.T) {
// 	domain := serverDomain
// 	proxyTo := ":25560"

// 	onlineStatusConfig := StatusConfig{VersionName: "online server"}
// 	offlineStatusConfig := StatusConfig{VersionName: "offline server"}
// 	proxyConfig := &ProxyConfig{DomainName: domain, ProxyTo: proxyTo, OnlineStatus: onlineStatusConfig, OfflineStatus: offlineStatusConfig}
// 	proxy := &Proxy{Config: proxyConfig}

// 	fmt.Printf("timeout time: %v\n", proxy.Timeout())

// 	tt := []struct {
// 		name          string
// 		online        bool
// 		ping          bool
// 		expectedError error
// 	}{
// 		{
// 			name:   "status online with ping",
// 			online: true,
// 			ping:   true,
// 		},
// 		{
// 			name:   "status offline with ping",
// 			online: false,
// 			ping:   true,
// 		},
// 		{
// 			name:   "status online without ping",
// 			online: true,
// 			ping:   false,
// 		},
// 		{
// 			name:   "status offline without ping",
// 			online: false,
// 			ping:   false,
// 		},
// 	}

// 	for _, tc := range tt {
// 		t.Run(tc.name, func(t *testing.T) {
// 			c1, c2 := net.Pipe()
// 			cServer := createTestConn(c1)
// 			cClient := createTestConn(c2)

// 			go func(c Conn) {
// 				// pk := serverHandshake(domain, 25565)
// 				// dialConfig := statusDialConfig{
// 				// 	conn:        c,
// 				// 	pk:          pk,
// 				// 	sendEndPing: true,
// 				// }
// 				// _, err := statusDial(dialConfig)
// 				// if err != nil {
// 				// 	fmt.Println(err)
// 				// }

// 				pk := serverHandshake(domain, 25565)
// 				sendHandshake(c, pk)

// 				receivedPk, err := c.ReadPacket()
// 				if err != nil {
// 					return
// 				}

// 				response, err := status.UnmarshalClientBoundResponse(receivedPk)
// 				if err != nil {
// 					return
// 				}

// 				res := &status.ResponseJSON{}
// 				json.Unmarshal([]byte(response.JSONResponse), &res)

// 				if !tc.ping {
// 					c.Close()
// 					return
// 				}

// 				//Send ping st
// 				pingPk := status.ServerBoundRequest{}.Marshal()
// 				if err := c.WritePacket(pingPk); err != nil {
// 					return
// 				}

// 				c.ReadPacket()
// 			}(cClient)

// 			err := proxy.handleStatusRequest(cServer, tc.online)
// 			if err != nil {
// 				if tc.expectedError == nil {
// 					t.Errorf("Didnt expect an error but got: %v", err)
// 				} else if tc.expectedError != err {
// 					t.Errorf("Expected an error of: %v, but got %v", tc.expectedError, err)
// 				}

// 			} else {
// 				if tc.expectedError != nil {
// 					t.Error("Did expect an error but got: nothing")
// 				}
// 			}

// 		})
// 	}
// }
