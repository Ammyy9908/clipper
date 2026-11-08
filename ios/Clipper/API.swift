import Foundation

// Production API URL with HTTPS and WebSocket (WSS) streaming
enum Config {
    static let baseURL = URL(string: "https://api.clipplay.video")!
    static let pollInterval: TimeInterval = 2
    static let pollTimeout: TimeInterval = 120 * 60 // 120 minutes for massive multi-hour videos
}

// MARK: - Wire types

struct ResolveResponse: Decodable {
    let title: String
    let uploader: String?
    let platform: String
    let durationSeconds: Double
    let thumbnail: String?
    let variants: [Variant]

    enum CodingKeys: String, CodingKey {
        case title, uploader, platform, thumbnail, variants
        case durationSeconds = "duration_seconds"
    }
}

struct Variant: Decodable, Hashable {
    let selector: String
    let label: String
    let height: Int
    let approxBytes: Int64?
    let audio: Bool

    enum CodingKeys: String, CodingKey {
        case selector, label, height, audio
        case approxBytes = "approx_bytes"
    }
}

struct JobRef: Decodable {
    let id: String
    let status: String
}

struct Job: Decodable {
    let id: String
    let title: String?
    let status: String
    let percent: Int?
    let speed: String?
    let eta: String?
    let stage: String?
    let error: String?
    let errorCode: String?
    let clips: [Clip]

    var isTerminal: Bool { status == "completed" || status == "failed" }

    enum CodingKeys: String, CodingKey {
        case id, title, status, percent, speed, eta, stage, error, clips
        case errorCode = "error_code"
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        title = try container.decodeIfPresent(String.self, forKey: .title)
        status = try container.decode(String.self, forKey: .status)
        percent = try container.decodeIfPresent(Int.self, forKey: .percent)
        speed = try container.decodeIfPresent(String.self, forKey: .speed)
        eta = try container.decodeIfPresent(String.self, forKey: .eta)
        stage = try container.decodeIfPresent(String.self, forKey: .stage)
        error = try container.decodeIfPresent(String.self, forKey: .error)
        errorCode = try container.decodeIfPresent(String.self, forKey: .errorCode)
        clips = try container.decodeIfPresent([Clip].self, forKey: .clips) ?? []
    }
}

struct Clip: Decodable {
    let index: Int
    let url: String
    let sizeBytes: Int64

    enum CodingKeys: String, CodingKey {
        case index, url
        case sizeBytes = "size_bytes"
    }
}

// MARK: - Auth & Quota Types

struct AuthGoogleResponse: Decodable {
    let token: String
    let expiresAt: String
    let user: UserProfile

    enum CodingKeys: String, CodingKey {
        case token, user
        case expiresAt = "expires_at"
    }
}

struct UserProfile: Codable {
    let email: String
    let name: String?
    let picture: String?
}

struct QuotaInfo: Decodable {
    let signedIn: Bool
    let authEnabled: Bool
    let limit: Int
    let signedInLimit: Int
    let remaining: Int?
    let used: Int?
    let resetsAt: String?
    let user: UserProfile?

    enum CodingKeys: String, CodingKey {
        case limit, remaining, used, user
        case signedIn = "signed_in"
        case authEnabled = "auth_enabled"
        case signedInLimit = "signed_in_limit"
        case resetsAt = "resets_at"
    }
}

// MARK: - Errors

enum APIError: LocalizedError {
    case server(String)
    case unavailable
    case blocked
    case platformDown
    case timedOut
    case badResponse
    case dailyLimit(String)

    var errorDescription: String? {
        switch self {
        case .server(let m):       return m
        case .unavailable:         return "This video is private, removed, or unavailable in your region."
        case .blocked:             return "Couldn't reach this video right now. Try again in a minute."
        case .platformDown:        return "This platform isn't working right now."
        case .timedOut:            return "This is taking longer than expected. Check back later."
        case .badResponse:         return "Something went wrong."
        case .dailyLimit(let m):   return m
        }
    }

    // The backend classifies failures; mirror that here so the UI can say
    // something true instead of "error".
    static func from(code: String?, message: String?) -> APIError {
        switch code {
        case "daily_limit":     return .dailyLimit(message ?? "Daily download limit reached. Sign in with Google to get 20 downloads/day.")
        case "unavailable":     return .unavailable
        case "blocked":         return .blocked
        case "extractor_stale": return .platformDown
        default:                return .server(message ?? "Something went wrong.")
        }
    }
}

private struct ErrorBody: Decodable {
    let error: String?
    let code: String?
}

// MARK: - Client

actor API {
    static let shared = API()

    private let session: URLSession = {
        let c = URLSessionConfiguration.default
        c.timeoutIntervalForRequest = 30
        c.waitsForConnectivity = true
        return URLSession(configuration: c)
    }()

    private let decoder = JSONDecoder()

    func resolve(url: String) async throws -> ResolveResponse {
        print("resolving:", url)
        return try await post("/v1/resolve", body: ["url": url])
    }

    func createJob(url: String, format: String?, audioOnly: Bool = false, segmentSeconds: Int = 0) async throws -> JobRef {
        let uid = await UserSession.shared.userID
        var body: [String: Any] = [
            "url": url,
            "segment_seconds": segmentSeconds,
            "audio_only": audioOnly,
            "user_id": uid
        ]
        if let format { body["format"] = format }
        return try await post("/v1/jobs", body: body)
    }

    func job(id: String) async throws -> Job {
        let cleanPath = "v1/jobs/\(id)"
        let url = URL(string: "\(Config.baseURL.absoluteString)/\(cleanPath)")!
        var req = URLRequest(url: url)
        req.httpMethod = "GET"
        return try await send(req)
    }

    func userJobs() async throws -> [Job] {
        let cleanPath = "v1/jobs"
        let url = URL(string: "\(Config.baseURL.absoluteString)/\(cleanPath)")!
        var req = URLRequest(url: url)
        req.httpMethod = "GET"
        return try await send(req)
    }

    // MARK: - Auth & Quota APIs

    func authenticateWithGoogle(idToken: String) async throws -> AuthGoogleResponse {
        return try await post("/v1/auth/google", body: ["credential": idToken])
    }

    func fetchQuota() async throws -> QuotaInfo {
        let cleanPath = "v1/me"
        let url = URL(string: "\(Config.baseURL.absoluteString)/\(cleanPath)")!
        var req = URLRequest(url: url)
        req.httpMethod = "GET"
        return try await send(req)
    }

    // MARK: - Real-Time Job Tracking (WebSocket + Polling Fallback)

    /// Waits for a job to reach a terminal state (completed/failed).
    /// Connects to ephemeral WebSocket stream (GET /v1/jobs/{id}/ws) first for immediate progress events.
    /// Automatically falls back to HTTP polling if WebSocket fails or disconnects.
    func waitForCompletion(id: String, onUpdate: @Sendable @escaping (Job) -> Void) async throws -> Job {
        do {
            return try await streamViaWebSocket(id: id, onUpdate: onUpdate)
        } catch {
            if Task.isCancelled { throw error }
            if let apiErr = error as? APIError, case .dailyLimit = apiErr { throw apiErr }
            if let apiErr = error as? APIError, case .server = apiErr { throw apiErr }
            if let apiErr = error as? APIError, case .unavailable = apiErr { throw apiErr }
            if let apiErr = error as? APIError, case .blocked = apiErr { throw apiErr }
            if let apiErr = error as? APIError, case .platformDown = apiErr { throw apiErr }

            print("WebSocket stream disconnected/failed for job \(id), falling back to polling: \(error)")
            return try await pollUntilCompletion(id: id, onUpdate: onUpdate)
        }
    }

    /// Listens to real-time progress via WebSocket (`/v1/jobs/{id}/ws`).
    private func streamViaWebSocket(id: String, onUpdate: @Sendable @escaping (Job) -> Void) async throws -> Job {
        guard let wsURL = makeWebSocketURL(forJobID: id) else {
            throw APIError.badResponse
        }

        let deviceID = await UserSession.shared.deviceID
        let userID = await UserSession.shared.userID
        let sessionToken = await UserSession.shared.sessionToken

        var req = URLRequest(url: wsURL)
        req.setValue(deviceID, forHTTPHeaderField: "X-Device-ID")
        req.setValue(userID, forHTTPHeaderField: "X-User-ID")
        if let token = sessionToken, !token.isEmpty {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let wsTask = session.webSocketTask(with: req)
        wsTask.resume()

        defer {
            wsTask.cancel(with: .normalClosure, reason: nil)
        }

        while !Task.isCancelled {
            let message = try await wsTask.receive()
            let data: Data
            switch message {
            case .data(let d):
                data = d
            case .string(let s):
                guard let d = s.data(using: .utf8) else { continue }
                data = d
            @unknown default:
                continue
            }

            guard let serverJob = try? decoder.decode(Job.self, from: data) else {
                continue
            }

            onUpdate(serverJob)

            if serverJob.isTerminal {
                if serverJob.status == "failed" {
                    throw APIError.from(code: serverJob.errorCode, message: serverJob.error)
                }
                return serverJob
            }
        }

        throw APIError.timedOut
    }

    /// Polling fallback that queries GET /v1/jobs/{id} until terminal state.
    private func pollUntilCompletion(id: String, onUpdate: @Sendable @escaping (Job) -> Void) async throws -> Job {
        let deadline = Date().addingTimeInterval(Config.pollTimeout)
        var consecutiveErrors = 0
        while Date() < deadline {
            if Task.isCancelled { throw CancellationError() }

            let serverJob: Job
            do {
                serverJob = try await job(id: id)
                consecutiveErrors = 0
            } catch {
                consecutiveErrors += 1
                if consecutiveErrors > 25 {
                    throw error
                }
                try await Task.sleep(nanoseconds: UInt64(Config.pollInterval * 1_000_000_000))
                continue
            }

            onUpdate(serverJob)

            if serverJob.isTerminal {
                if serverJob.status == "failed" {
                    throw APIError.from(code: serverJob.errorCode, message: serverJob.error)
                }
                return serverJob
            }

            try await Task.sleep(nanoseconds: UInt64(Config.pollInterval * 1_000_000_000))
        }
        throw APIError.timedOut
    }

    private func makeWebSocketURL(forJobID id: String) -> URL? {
        var components = URLComponents(url: Config.baseURL, resolvingAgainstBaseURL: true)
        if components?.scheme == "https" {
            components?.scheme = "wss"
        } else {
            components?.scheme = "ws"
        }
        let cleanPath = "v1/jobs/\(id)/ws"
        if let currentPath = components?.path, !currentPath.isEmpty && currentPath != "/" {
            components?.path = "\(currentPath.trimmingCharacters(in: CharacterSet(charactersIn: "/")))/\(cleanPath)"
        } else {
            components?.path = "/\(cleanPath)"
        }
        return components?.url
    }

    // MARK: plumbing

    private func post<T: Decodable>(_ path: String, body: [String: Any]) async throws -> T {
        let cleanPath = path.hasPrefix("/") ? String(path.dropFirst()) : path
        let url = URL(string: "\(Config.baseURL.absoluteString)/\(cleanPath)")!
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: body)
        return try await send(req)
    }

    private func send<T: Decodable>(_ req: URLRequest) async throws -> T {
        let deviceID = await UserSession.shared.deviceID
        let userID = await UserSession.shared.userID
        let sessionToken = await UserSession.shared.sessionToken

        var mutableReq = req
        mutableReq.setValue(deviceID, forHTTPHeaderField: "X-Device-ID")
        mutableReq.setValue(userID, forHTTPHeaderField: "X-User-ID")
        if let token = sessionToken, !token.isEmpty {
            mutableReq.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        print("API [\(mutableReq.httpMethod ?? "GET")] \(mutableReq.url?.absoluteString ?? "")")
        let (data, response) = try await session.data(for: mutableReq)
        guard let http = response as? HTTPURLResponse else { throw APIError.badResponse }

        guard (200..<300).contains(http.statusCode) else {
            let rawStr = String(data: data, encoding: .utf8) ?? ""
            print("API Error [\(http.statusCode)]: \(rawStr)")
            let body = try? decoder.decode(ErrorBody.self, from: data)
            throw APIError.from(code: body?.code, message: body?.error)
        }
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            print("API Decode Error: \(error)")
            throw APIError.badResponse
        }
    }
}
