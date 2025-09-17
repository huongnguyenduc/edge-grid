// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract JobEscrow is Ownable {
    using SafeERC20 for IERC20;

    enum Status { None, Open, Delivered, Settled, Canceled }

    struct Job {
        address client;
        address node;
        address token;
        uint256 amount;
        uint256 deadline;
        bytes32 resultHash;
        Status status;
        string modelId;
        uint256 createdAt;
    }

    mapping(uint256 => Job) public jobs;
    mapping(address => mapping(uint256 => bool)) public usedNonces;
    uint256 public nextJobId = 1;

    event JobCreated(
        uint256 indexed jobId,
        address indexed client,
        address indexed node,
        address token,
        uint256 amount,
        uint256 deadline,
        string modelId
    );
    event JobSettled(uint256 indexed jobId, address indexed node, bytes32 resultHash);
    event JobCanceled(uint256 indexed jobId);

    error JobNotFound();
    error JobExpired();
    error InvalidSignature();
    error NonceAlreadyUsed();
    error InvalidAmount();
    error TransferFailed();

    bytes32 public constant RESULT_TYPEHASH = keccak256(
        "Result(uint256 jobId,bytes32 resultHash,uint256 computeMs,uint256 score,uint256 nonce,uint256 deadline)"
    );

    constructor() Ownable(msg.sender) {}

    function createJob(
        address node,
        address token,
        uint256 amount,
        uint256 deadline,
        string calldata modelId
    ) external payable returns (uint256 jobId) {
        if (amount == 0) revert InvalidAmount();
        if (deadline <= block.timestamp) revert JobExpired();

        jobId = nextJobId++;
        
        jobs[jobId] = Job({
            client: msg.sender,
            node: node,
            token: token,
            amount: amount,
            deadline: deadline,
            resultHash: bytes32(0),
            status: Status.Open,
            modelId: modelId,
            createdAt: block.timestamp
        });

        // Handle payment
        if (token == address(0)) {
            // Native token
            if (msg.value != amount) revert InvalidAmount();
        } else {
            // ERC20 token
            IERC20(token).safeTransferFrom(msg.sender, address(this), amount);
        }

        emit JobCreated(jobId, msg.sender, node, token, amount, deadline, modelId);
    }

    function settleJob(
        uint256 jobId,
        bytes32 resultHash,
        uint256 computeMs,
        uint256 score,
        uint256 nonce,
        uint256 sigDeadline,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) external {
        Job storage job = jobs[jobId];
        if (job.status != Status.Open) revert JobNotFound();
        if (block.timestamp > job.deadline) revert JobExpired();
        if (block.timestamp > sigDeadline) revert JobExpired();
        if (usedNonces[job.node][nonce]) revert NonceAlreadyUsed();

        // Verify signature
        bytes32 structHash = keccak256(
            abi.encode(
                RESULT_TYPEHASH,
                jobId,
                resultHash,
                computeMs,
                score,
                nonce,
                sigDeadline
            )
        );

        bytes32 domainSeparator = keccak256(
            abi.encode(
                keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
                keccak256(bytes("EdgeAI")),
                keccak256(bytes("1")),
                block.chainid,
                address(this)
            )
        );

        bytes32 hash = keccak256(abi.encodePacked("\x19\x01", domainSeparator, structHash));
        address signer = ecrecover(hash, v, r, s);

        if (signer != job.node) revert InvalidSignature();

        // Mark nonce as used
        usedNonces[job.node][nonce] = true;

        // Update job
        job.resultHash = resultHash;
        job.status = Status.Settled;

        // Transfer payment to node
        if (job.token == address(0)) {
            (bool success,) = job.node.call{value: job.amount}("");
            if (!success) revert TransferFailed();
        } else {
            IERC20(job.token).safeTransfer(job.node, job.amount);
        }

        emit JobSettled(jobId, job.node, resultHash);
    }

    function cancelExpired(uint256 jobId) external {
        Job storage job = jobs[jobId];
        if (job.status != Status.Open) revert JobNotFound();
        if (block.timestamp <= job.deadline) revert JobExpired();

        job.status = Status.Canceled;

        // Refund to client
        if (job.token == address(0)) {
            (bool success,) = job.client.call{value: job.amount}("");
            if (!success) revert TransferFailed();
        } else {
            IERC20(job.token).safeTransfer(job.client, job.amount);
        }

        emit JobCanceled(jobId);
    }
}
