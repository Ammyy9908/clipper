import Foundation
#if canImport(UIKit)
import UIKit
#endif

/// Encapsulates a persistent record of an in-flight job so that if the user
/// closes the app or iOS suspends execution, the download can be resumed
/// and finished automatically upon reopening.
struct PendingDownload: Codable, Identifiable {
    let id: String
    let url: String
    let title: String
    let thumbnail: String?
    let isAudio: Bool
    let isSplit: Bool?
    let createdAt: Date
}

/// One in-flight (or finished) download, tracked independently so its status
/// updates without blocking the rest of the Download tab.
@MainActor
final class DownloadJob: ObservableObject, Identifiable {
    let id: String
    var serverJobID: String?
    let title: String
    let thumbnail: String?
    let isAudio: Bool
    let isSplit: Bool

    enum Status: Equatable {
        case queued
        case working(String)   // server-side: downloading / processing / splitting
        case saving(Double)    // device-side: pulling the file down, 0...1
        case done
        case failed(String)
    }

    @Published var status: Status = .queued

    var statusText: String {
        switch status {
        case .queued: return "Queued"
        case .working(let stage): return stage
        case .saving(let p): return "Saving to device \(Int(p * 100))%"
        case .done:
            if isAudio { return "Saved as MP3" }
            if isSplit { return "Split & saved" }
            return "Saved"
        case .failed(let message): return message
        }
    }

    var progress: Double? {
        switch status {
        case .saving(let p):
            return p
        case .working(let text):
            if let match = text.range(of: #"\d+(?:\.\d+)?%"#, options: .regularExpression) {
                let percentStr = text[match].dropLast()
                if let val = Double(percentStr) {
                    return min(max(val / 100.0, 0.0), 1.0)
                }
            }
            return nil
        case .done:
            return 1.0
        default:
            return nil
        }
    }

    var isFinished: Bool {
        switch status {
        case .done, .failed: return true
        default: return false
        }
    }

    init(id: String = UUID().uuidString, serverJobID: String? = nil, title: String, thumbnail: String?, isAudio: Bool, isSplit: Bool = false) {
        self.id = id
        self.serverJobID = serverJobID
        self.title = title
        self.thumbnail = thumbnail
        self.isAudio = isAudio
        self.isSplit = isSplit
    }
}

/// Runs every queued download concurrently, managing persistent storage,
/// background execution tokens, and seamless rehydration on app restarts.
@MainActor
final class DownloadQueue: ObservableObject {
    static let shared = DownloadQueue()

    @Published private(set) var jobs: [DownloadJob] = []

    var activeJobs: [DownloadJob] {
        jobs.filter { !$0.isFinished }
    }
    private let pendingKey = "clipper_pending_downloads_v1"

    private init() {
        rehydrate()
    }

    // MARK: - Persistence

    private func loadPending() -> [PendingDownload] {
        guard let data = UserDefaults.standard.data(forKey: pendingKey),
              let list = try? JSONDecoder().decode([PendingDownload].self, from: data)
        else { return [] }
        return list
    }

    private func savePending(_ item: PendingDownload) {
        var list = loadPending()
        list.removeAll { $0.id == item.id }
        list.append(item)
        if let data = try? JSONEncoder().encode(list) {
            UserDefaults.standard.set(data, forKey: pendingKey)
        }
    }

    private func removePending(id: String) {
        var list = loadPending()
        list.removeAll { $0.id == id }
        if let data = try? JSONEncoder().encode(list) {
            UserDefaults.standard.set(data, forKey: pendingKey)
        }
    }

    // MARK: - Queue Actions

    func start(url: String, title: String, thumbnail: String?, format: String?, isAudio: Bool, segmentSeconds: Int) {
        JobNotificationManager.shared.requestAuthorizationIfNeeded()
        let isSplit = segmentSeconds > 0 && !isAudio
        let displayTitle = title.isEmpty ? url : title
        let job = DownloadJob(title: displayTitle, thumbnail: thumbnail, isAudio: isAudio, isSplit: isSplit)
        jobs.insert(job, at: 0)

        Task {
            await run(job, url: url, title: displayTitle, format: format, isAudio: isAudio, isSplit: isSplit, segmentSeconds: segmentSeconds)
        }
    }

    /// Rechecks all pending server jobs on launch or when the app enters foreground.
    func rehydrate() {
        let pendingList = loadPending()
        let cutoff = Date().addingTimeInterval(-2 * 3600)
        for item in pendingList {
            // Prune jobs older than 2 hours automatically
            if item.createdAt < cutoff {
                removePending(id: item.id)
                continue
            }
            if jobs.contains(where: { $0.serverJobID == item.id || $0.id == item.id }) {
                continue
            }
            let job = DownloadJob(
                id: item.id,
                serverJobID: item.id,
                title: item.title,
                thumbnail: item.thumbnail,
                isAudio: item.isAudio,
                isSplit: item.isSplit ?? false
            )
            jobs.insert(job, at: 0)
            Task {
                await resumePendingJob(job, item: item)
            }
        }

        // Also sync active user jobs from backend
        Task {
            do {
                let serverJobs = try await API.shared.userJobs()
                for sJob in serverJobs where !sJob.isTerminal {
                    if !self.jobs.contains(where: { $0.serverJobID == sJob.id || $0.id == sJob.id }) {
                        let job = DownloadJob(id: sJob.id, serverJobID: sJob.id, title: sJob.title ?? "Downloading Video", thumbnail: nil, isAudio: false, isSplit: false)
                        self.jobs.insert(job, at: 0)
                        Task {
                            await self.resumePendingJob(job, item: PendingDownload(
                                id: sJob.id,
                                url: "",
                                title: sJob.title ?? "Downloading Video",
                                thumbnail: nil,
                                isAudio: false,
                                isSplit: false,
                                createdAt: Date()
                            ))
                        }
                    }
                }
            } catch {
                print("Failed to sync server jobs: \(error)")
            }
        }
    }

    func remove(_ job: DownloadJob) {
        jobs.removeAll { $0.id == job.id }
        removePending(id: job.id)
        if let sid = job.serverJobID {
            removePending(id: sid)
        }
    }

    // MARK: - Execution Engine

    private func run(_ job: DownloadJob, url: String, title: String, format: String?, isAudio: Bool, isSplit: Bool, segmentSeconds: Int) async {
        #if canImport(UIKit)
        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "clipper_download_\(job.id)") {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        #endif

        do {
            let ref = try await API.shared.createJob(
                url: url,
                format: format,
                audioOnly: isAudio,
                segmentSeconds: segmentSeconds
            )

            job.serverJobID = ref.id

            // Persist the in-flight job so app restarts can recover it
            savePending(PendingDownload(
                id: ref.id,
                url: url,
                title: title,
                thumbnail: job.thumbnail,
                isAudio: isAudio,
                isSplit: isSplit,
                createdAt: Date()
            ))

            await finishJobExecution(job: job, serverJobID: ref.id, title: title, isAudio: isAudio, isSplit: isSplit)
        } catch {
            let message = error.localizedDescription
            job.status = .failed(message)
            if let sid = job.serverJobID {
                removePending(id: sid)
            }
            removePending(id: job.id)
            JobNotificationManager.shared.notifyFailed(title: job.title, message: message)
            scheduleAutoRemove(job, after: 6)
        }
    }

    private func resumePendingJob(_ job: DownloadJob, item: PendingDownload) async {
        #if canImport(UIKit)
        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "clipper_resume_\(job.id)") {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        #endif

        await finishJobExecution(job: job, serverJobID: item.id, title: item.title, isAudio: item.isAudio, isSplit: item.isSplit ?? false)
    }

    private func finishJobExecution(job: DownloadJob, serverJobID: String, title: String, isAudio: Bool, isSplit: Bool) async {
        do {
            let serverJob = try await API.shared.waitForCompletion(id: serverJobID) { serverJob in
                Task { @MainActor in
                    let label: String
                    switch serverJob.status {
                    case "downloading":
                        var text = isAudio ? "Downloading audio" : "Downloading"
                        if let percent = serverJob.percent, percent > 0 {
                            text += " (\(percent)%)"
                        }
                        if let speed = serverJob.speed, !speed.isEmpty {
                            text += " • \(speed)"
                        }
                        label = text
                    case "processing":
                        if isSplit {
                            label = "Splitting video"
                        } else if isAudio {
                            label = "Converting audio"
                        } else {
                            label = "Processing video"
                        }
                    default:
                        label = "Queued"
                    }
                    job.status = .working(label)
                }
            }

            guard !serverJob.clips.isEmpty else {
                job.status = .failed("No file came back.")
                removePending(id: serverJobID)
                JobNotificationManager.shared.notifyFailed(title: job.title, message: "No file came back.")
                scheduleAutoRemove(job, after: 6)
                return
            }

            job.status = .saving(0)

            var artworkFilename: String?
            if let thumb = job.thumbnail, let thumbURL = URL(string: thumb) {
                let name = "\(serverJobID).jpg"
                if await PhotoSaver.downloadArtwork(from: thumbURL, filename: name) {
                    artworkFilename = name
                }
            }

            let clips = serverJob.clips.sorted { $0.index < $1.index }
            let isSplit = clips.count > 1
            let totalBytes = max(clips.reduce(Int64(0)) { $0 + $1.sizeBytes }, 1)
            var completedBytes: Int64 = 0
            let baseTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)

            for clip in clips {
                guard let remote = URL(string: clip.url) else { continue }

                let remoteName = URL(fileURLWithPath: clip.url.split(separator: "?").first.map(String.init) ?? clip.url).lastPathComponent
                let ext = (remoteName as NSString).pathExtension
                let defaultExt = isAudio ? "mp3" : "mp4"
                let base = isSplit ? "\(serverJobID)_\(clip.index)" : serverJobID
                let safeName = "\(base)\(ext.isEmpty ? ".\(defaultExt)" : ".\(ext)")"

                let local = try await PhotoSaver.downloadToLibrary(from: remote, filename: safeName) { p in
                    Task { @MainActor in
                        let clipBytes = Double(clip.sizeBytes) * p
                        job.status = .saving((Double(completedBytes) + clipBytes) / Double(totalBytes))
                    }
                }
                completedBytes += clip.sizeBytes

                let size = (try? FileManager.default.attributesOfItem(atPath: local.path)[.size] as? Int64) ?? clip.sizeBytes
                let name = baseTitle.isEmpty ? safeName : baseTitle
                let clipTitle = isSplit ? "\(name) (Part \(clip.index + 1))" : name

                LibraryStore.shared.add(SavedClip(
                    id: base,
                    title: clipTitle,
                    filename: safeName,
                    sizeBytes: size,
                    createdAt: Date(),
                    artworkFilename: artworkFilename,
                    groupID: isSplit ? serverJobID : nil,
                    groupTitle: isSplit ? name : nil,
                    partIndex: isSplit ? clip.index : nil
                ))

                if LibraryStore.shared.autoSave && !isAudio {
                    try? await PhotoSaver.addToPhotos(fileURL: local)
                }

                if CloudStorageManager.shared.autoUploadToCloud {
                    let saved = SavedClip(
                        id: base,
                        title: clipTitle,
                        filename: safeName,
                        sizeBytes: size,
                        createdAt: Date(),
                        artworkFilename: artworkFilename,
                        groupID: isSplit ? serverJobID : nil,
                        groupTitle: isSplit ? name : nil,
                        partIndex: isSplit ? clip.index : nil
                    )
                    Task {
                        await CloudStorageManager.shared.upload(clip: saved)
                    }
                }
            }

            removePending(id: serverJobID)
            job.status = .done
            await UserSession.shared.refreshQuota()
            JobNotificationManager.shared.notifyDone(title: job.title, isAudio: isAudio, isSplit: isSplit)
            scheduleAutoRemove(job, after: 3)
        } catch {
            let message = error.localizedDescription
            job.status = .failed(message)
            removePending(id: serverJobID)
            JobNotificationManager.shared.notifyFailed(title: job.title, message: message)
            scheduleAutoRemove(job, after: 6)
        }
    }

    private func scheduleAutoRemove(_ job: DownloadJob, after seconds: Double) {
        Task {
            try? await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            remove(job)
        }
    }
}
