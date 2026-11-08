import Foundation
import Security
import GoogleSignIn
#if canImport(UIKit)
import UIKit
#endif

/// Manages a stable, unique device identifier and authenticated Google User Session
/// with daily quota tracking (3 for anonymous, 20 for authenticated).
@MainActor
final class UserSession: ObservableObject {
    static let shared = UserSession()

    private let keychainService = "com.clipper.app"
    private let keychainDeviceAccount = "device_user_id"
    private let keychainTokenAccount = "auth_session_token"
    private let userDefaultsDeviceKey = "clipper_device_user_id"
    private let userDefaultsUserKey = "clipper_cached_user_profile"

    private(set) var deviceID: String
    private(set) var sessionToken: String?

    @Published private(set) var isSignedIn: Bool = false
    @Published private(set) var userEmail: String?
    @Published private(set) var userName: String?
    @Published private(set) var userPicture: String?
    @Published private(set) var quota: QuotaInfo?
    @Published private(set) var isSigningIn: Bool = false
    @Published var authError: String?

    var userID: String {
        if let email = userEmail, isSignedIn {
            return "google:\(email)"
        }
        return deviceID
    }

    private init() {
        // 1. Initialize stable Device ID
        if let stored = UserSession.readKeychain(service: keychainService, account: keychainDeviceAccount), !stored.isEmpty {
            self.deviceID = stored
            UserDefaults.standard.set(stored, forKey: userDefaultsDeviceKey)
        } else if let cached = UserDefaults.standard.string(forKey: userDefaultsDeviceKey), !cached.isEmpty {
            self.deviceID = cached
            _ = UserSession.saveKeychain(service: keychainService, account: keychainDeviceAccount, value: cached)
        } else {
            let newID = UUID().uuidString.lowercased()
            self.deviceID = newID
            UserDefaults.standard.set(newID, forKey: userDefaultsDeviceKey)
            _ = UserSession.saveKeychain(service: keychainService, account: keychainDeviceAccount, value: newID)
        }

        // 2. Restore Session Token & Cached User Profile
        if let token = UserSession.readKeychain(service: keychainService, account: keychainTokenAccount), !token.isEmpty {
            self.sessionToken = token
            self.isSignedIn = true
            if let data = UserDefaults.standard.data(forKey: userDefaultsUserKey),
               let user = try? JSONDecoder().decode(UserProfile.self, from: data) {
                self.userEmail = user.email
                self.userName = user.name
                self.userPicture = user.picture
            }
        }

        // 3. Refresh Quota from API asynchronously
        Task {
            await self.refreshQuota()
        }
    }

    // MARK: - Quota Management

    func refreshQuota() async {
        do {
            let info = try await API.shared.fetchQuota()
            self.quota = info
            self.isSignedIn = info.signedIn
            if let user = info.user {
                self.userEmail = user.email
                self.userName = user.name
                self.userPicture = user.picture
                if let data = try? JSONEncoder().encode(user) {
                    UserDefaults.standard.set(data, forKey: self.userDefaultsUserKey)
                }
            }
        } catch {
            print("Failed to fetch quota: \(error)")
        }
    }

    // MARK: - Google Authentication

    func signInWithGoogle() {
        guard let presenter = Self.rootViewController() else {
            self.authError = "Could not find root view controller to present Google Sign-In."
            return
        }

        self.isSigningIn = true
        self.authError = nil

        let clientID = "87309036891-9lvf23qk0umuqmbohc36rin8sveqsfg4.apps.googleusercontent.com"
        let serverClientID = "87309036891-1niie4klco7bmftqt2umsctpm9m6ijca.apps.googleusercontent.com"
        GIDSignIn.sharedInstance.configuration = GIDConfiguration(clientID: clientID, serverClientID: serverClientID)

        GIDSignIn.sharedInstance.signIn(
            withPresenting: presenter,
            hint: nil,
            additionalScopes: ["email", "profile"]
        ) { [weak self] result, error in
            Task { @MainActor [weak self] in
                guard let self = self else { return }
                self.isSigningIn = false

                if let error = error {
                    let nsError = error as NSError
                    if nsError.code == -5 { // kGIDSignInErrorCodeCanceled
                        return // User intentionally cancelled
                    }
                    self.authError = error.localizedDescription
                    return
                }

                guard let idToken = result?.user.idToken?.tokenString else {
                    self.authError = "Google did not provide a valid ID token."
                    return
                }

                await self.exchangeGoogleToken(idToken)
            }
        }
    }

    private func exchangeGoogleToken(_ idToken: String) async {
        do {
            let authResponse = try await API.shared.authenticateWithGoogle(idToken: idToken)
            self.sessionToken = authResponse.token
            _ = UserSession.saveKeychain(service: keychainService, account: keychainTokenAccount, value: authResponse.token)

            self.isSignedIn = true
            self.userEmail = authResponse.user.email
            self.userName = authResponse.user.name
            self.userPicture = authResponse.user.picture

            if let data = try? JSONEncoder().encode(authResponse.user) {
                UserDefaults.standard.set(data, forKey: userDefaultsUserKey)
            }

            // Immediately reload quota to reflect the 20 downloads/day tier
            await self.refreshQuota()
        } catch {
            self.authError = error.localizedDescription
        }
    }

    func signOut() {
        GIDSignIn.sharedInstance.signOut()
        UserSession.deleteKeychain(service: keychainService, account: keychainTokenAccount)
        UserDefaults.standard.removeObject(forKey: userDefaultsUserKey)

        self.sessionToken = nil
        self.isSignedIn = false
        self.userEmail = nil
        self.userName = nil
        self.userPicture = nil

        Task {
            await self.refreshQuota()
        }
    }

    // MARK: - Root Controller Helper

    private static func rootViewController() -> UIViewController? {
        #if canImport(UIKit)
        return UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap { $0.windows }
            .first { $0.isKeyWindow }?
            .rootViewController
        #else
        return nil
        #endif
    }

    // MARK: - Keychain Helpers

    private static func saveKeychain(service: String, account: String, value: String) -> Bool {
        guard let data = value.data(using: .utf8) else { return false }

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock
        ]

        SecItemDelete(query as CFDictionary)
        let status = SecItemAdd(query as CFDictionary, nil)
        return status == errSecSuccess
    }

    private static func readKeychain(service: String, account: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        guard status == errSecSuccess, let data = item as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private static func deleteKeychain(service: String, account: String) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        SecItemDelete(query as CFDictionary)
    }
}
