package domiconabi

const L1DomiconCommitment = `[
  {
    "inputs": [],
    "stateMutability": "nonpayable",
    "type": "constructor"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "a",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "b",
        "type": "address"
      },
      {
        "indexed": false,
        "internalType": "uint256",
        "name": "index",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "bytes",
        "name": "commitment",
        "type": "bytes"
      }
    ],
    "name": "FinalizeSubmitCommitment",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": false,
        "internalType": "uint8",
        "name": "version",
        "type": "uint8"
      }
    ],
    "name": "Initialized",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "internalType": "address",
        "name": "A",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "address",
        "name": "B",
        "type": "address"
      },
      {
        "indexed": true,
        "internalType": "uint256",
        "name": "index",
        "type": "uint256"
      },
      {
        "indexed": false,
        "internalType": "bytes",
        "name": "commitment",
        "type": "bytes"
      }
    ],
    "name": "SendDACommitment",
    "type": "event"
  },
  {
    "inputs": [],
    "name": "MESSENGER",
    "outputs": [
      {
        "internalType": "contract CrossDomainMessenger",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "OTHER_COMMITMENT",
    "outputs": [
      {
        "internalType": "contract DomiconCommitment",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "uint256",
        "name": "_index",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "_length",
        "type": "uint256"
      },
      {
        "internalType": "address",
        "name": "_user",
        "type": "address"
      },
      {
        "internalType": "bytes",
        "name": "_sign",
        "type": "bytes"
      },
      {
        "internalType": "bytes",
        "name": "_commitment",
        "type": "bytes"
      }
    ],
    "name": "SubmitCommitment",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "a",
        "type": "address"
      },
      {
        "internalType": "address",
        "name": "b",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "index",
        "type": "uint256"
      },
      {
        "internalType": "bytes",
        "name": "commitment",
        "type": "bytes"
      }
    ],
    "name": "finalizeSubmitCommitment",
    "outputs": [],
    "stateMutability": "payable",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      }
    ],
    "name": "indices",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "contract CrossDomainMessenger",
        "name": "_messenger",
        "type": "address"
      }
    ],
    "name": "initialize",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "messenger",
    "outputs": [
      {
        "internalType": "contract CrossDomainMessenger",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "otherCommitment",
    "outputs": [
      {
        "internalType": "contract DomiconCommitment",
        "name": "",
        "type": "address"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {
        "internalType": "address",
        "name": "",
        "type": "address"
      },
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "submits",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "index",
        "type": "uint256"
      },
      {
        "internalType": "uint256",
        "name": "length",
        "type": "uint256"
      },
      {
        "internalType": "address",
        "name": "user",
        "type": "address"
      },
      {
        "internalType": "address",
        "name": "broadcaster",
        "type": "address"
      },
      {
        "internalType": "bytes",
        "name": "sign",
        "type": "bytes"
      },
      {
        "internalType": "bytes",
        "name": "commitment",
        "type": "bytes"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "version",
    "outputs": [
      {
        "internalType": "string",
        "name": "",
        "type": "string"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  }
]`

const DomiconNodes = `[
	{
	  "inputs": [

	  ],
	  "stateMutability": "nonpayable",
	  "type": "constructor"
	},
	{
	  "anonymous": false,
	  "inputs": [
		{
		  "indexed": true,
		  "internalType": "address",
		  "name": "add",
		  "type": "address"
		},
		{
		  "indexed": false,
		  "internalType": "string",
		  "name": "rpc",
		  "type": "string"
		},
		{
		  "indexed": false,
		  "internalType": "string",
		  "name": "name",
		  "type": "string"
		},
		{
		  "indexed": false,
		  "internalType": "uint256",
		  "name": "stakedTokens",
		  "type": "uint256"
		}
	  ],
	  "name": "BroadcastNode",
	  "type": "event"
	},
	{
	  "anonymous": false,
	  "inputs": [
		{
		  "components": [
			{
			  "internalType": "address",
			  "name": "add",
			  "type": "address"
			},
			{
			  "internalType": "string",
			  "name": "rpc",
			  "type": "string"
			},
			{
			  "internalType": "string",
			  "name": "name",
			  "type": "string"
			},
			{
			  "internalType": "uint256",
			  "name": "stakedTokens",
			  "type": "uint256"
			},
			{
			  "internalType": "uint256",
			  "name": "index",
			  "type": "uint256"
			}
		  ],
		  "indexed": false,
		  "internalType": "structDomiconNode.NodeInfo",
		  "name": "nodeInfo",
		  "type": "tuple"
		}
	  ],
	  "name": "FinalizeBroadcastNode",
	  "type": "event"
	},
	{
	  "anonymous": false,
	  "inputs": [
		{
		  "indexed": false,
		  "internalType": "uint8",
		  "name": "version",
		  "type": "uint8"
		}
	  ],
	  "name": "Initialized",
	  "type": "event"
	},
	{
	  "inputs": [

	  ],
	  "name": "BROADCAST_NODES",
	  "outputs": [
		{
		  "internalType": "address[]",
		  "name": "",
		  "type": "address[]"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "addr",
		  "type": "address"
		}
	  ],
	  "name": "IsNodeBroadcast",
	  "outputs": [
		{
		  "internalType": "bool",
		  "name": "",
		  "type": "bool"
		}
	  ],
	  "stateMutability": "nonpayable",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "MESSENGER",
	  "outputs": [
		{
		  "internalType": "contractCrossDomainMessenger",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "OTHER_DOMICON_NODE",
	  "outputs": [
		{
		  "internalType": "contractDomiconNode",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "_address",
		  "type": "address"
		},
		{
		  "internalType": "string",
		  "name": "_rpc",
		  "type": "string"
		},
		{
		  "internalType": "string",
		  "name": "_name",
		  "type": "string"
		},
		{
		  "internalType": "uint256",
		  "name": "_stakedTokens",
		  "type": "uint256"
		}
	  ],
	  "name": "RegisterBroadcastNode",
	  "outputs": [

	  ],
	  "stateMutability": "nonpayable",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "STORAGE_NODES",
	  "outputs": [
		{
		  "internalType": "address[]",
		  "name": "",
		  "type": "address[]"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "uint256",
		  "name": "",
		  "type": "uint256"
		}
	  ],
	  "name": "broadcastNodeList",
	  "outputs": [
		{
		  "internalType": "address",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "name": "broadcastingNodes",
	  "outputs": [
		{
		  "internalType": "address",
		  "name": "add",
		  "type": "address"
		},
		{
		  "internalType": "string",
		  "name": "rpc",
		  "type": "string"
		},
		{
		  "internalType": "string",
		  "name": "name",
		  "type": "string"
		},
		{
		  "internalType": "uint256",
		  "name": "stakedTokens",
		  "type": "uint256"
		},
		{
		  "internalType": "uint256",
		  "name": "index",
		  "type": "uint256"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "node",
		  "type": "address"
		},
		{
		  "components": [
			{
			  "internalType": "address",
			  "name": "add",
			  "type": "address"
			},
			{
			  "internalType": "string",
			  "name": "rpc",
			  "type": "string"
			},
			{
			  "internalType": "string",
			  "name": "name",
			  "type": "string"
			},
			{
			  "internalType": "uint256",
			  "name": "stakedTokens",
			  "type": "uint256"
			},
			{
			  "internalType": "uint256",
			  "name": "index",
			  "type": "uint256"
			}
		  ],
		  "internalType": "structDomiconNode.NodeInfo",
		  "name": "nodeInfo",
		  "type": "tuple"
		}
	  ],
	  "name": "finalizeBroadcastNode",
	  "outputs": [

	  ],
	  "stateMutability": "payable",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "contractCrossDomainMessenger",
		  "name": "_messenger",
		  "type": "address"
		}
	  ],
	  "name": "initialize",
	  "outputs": [

	  ],
	  "stateMutability": "nonpayable",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "messenger",
	  "outputs": [
		{
		  "internalType": "contractCrossDomainMessenger",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "otherDomiconNode",
	  "outputs": [
		{
		  "internalType": "contractDomiconNode",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "uint256",
		  "name": "",
		  "type": "uint256"
		}
	  ],
	  "name": "storageNodeList",
	  "outputs": [
		{
		  "internalType": "address",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [
		{
		  "internalType": "address",
		  "name": "",
		  "type": "address"
		}
	  ],
	  "name": "storageNodes",
	  "outputs": [
		{
		  "internalType": "address",
		  "name": "add",
		  "type": "address"
		},
		{
		  "internalType": "string",
		  "name": "rpc",
		  "type": "string"
		},
		{
		  "internalType": "string",
		  "name": "name",
		  "type": "string"
		},
		{
		  "internalType": "uint256",
		  "name": "stakedTokens",
		  "type": "uint256"
		},
		{
		  "internalType": "uint256",
		  "name": "index",
		  "type": "uint256"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	},
	{
	  "inputs": [

	  ],
	  "name": "version",
	  "outputs": [
		{
		  "internalType": "string",
		  "name": "",
		  "type": "string"
		}
	  ],
	  "stateMutability": "view",
	  "type": "function"
	}
  ]`
