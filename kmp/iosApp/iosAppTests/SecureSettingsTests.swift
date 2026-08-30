import ComposeApp
import XCTest

/// The wiring on the platform it was written for, which nothing else measures: `secureSettings`
/// running the fresh-install guard over a real keychain and a real `NSUserDefaults`.
///
/// `FreshInstallGuardTest` proves the rule over two fakes. Deleting the call that applies it on
/// Apple survived that suite entirely, and the price of the gap is a patient inheriting the
/// session of whoever had the device before them.
final class SecureSettingsTests: XCTestCase {
    private let store = "test-\(UUID().uuidString)"
    private var service: String { "app.cadence.\(store)" }
    private var marker: String { "\(FreshInstallGuardKt.INSTALL_MARKER_KEY).\(store)" }

    override func tearDown() {
        deleteKeychainItem()
        UserDefaults.standard.removeObject(forKey: marker)
        super.tearDown()
    }

    /// A store the previous installation left behind: an item in the keychain, and no marker in
    /// the defaults, because deleting the app clears those and does not clear the keychain.
    func testAStoreOutlivingItsInstallationIsWipedOnFirstUse() {
        writeKeychainItem()
        UserDefaults.standard.removeObject(forKey: marker)

        let settings = SecureSettings_iosKt.secureSettings(name: store)

        XCTAssertEqual(settings.size, 0)
        XCTAssertNil(readKeychainItem())
        XCTAssertTrue(UserDefaults.standard.bool(forKey: marker))
    }

    private func query() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "store",
        ]
    }

    private func writeKeychainItem() {
        deleteKeychainItem()
        var item = query()
        item[kSecValueData as String] = Data("8:rt-token4:rt-1".utf8)
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        item[kSecAttrSynchronizable as String] = false
        XCTAssertEqual(SecItemAdd(item as CFDictionary, nil), errSecSuccess)
    }

    private func readKeychainItem() -> Data? {
        var item = query()
        item[kSecReturnData as String] = true
        var found: CFTypeRef?
        guard SecItemCopyMatching(item as CFDictionary, &found) == errSecSuccess else { return nil }
        return found as? Data
    }

    private func deleteKeychainItem() {
        SecItemDelete(query() as CFDictionary)
    }
}
