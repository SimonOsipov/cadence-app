import ComposeApp
import XCTest

/// Hosted by the app because a keychain needs an app identity — `KeychainReachabilityTest` in
/// `shared/src/iosTest` pins the refusal that put them here. These are the assertions it
/// displaced, running where that identity exists.
final class KeychainVaultTests: XCTestCase {
    private let service = "app.cadence.test-vault"
    private lazy var vault = KeychainVault(service: service)

    override func tearDown() {
        vault.wipe()
        super.tearDown()
    }

    func testWhatIsWrittenComesBack() {
        XCTAssertTrue(vault.write(bytes: bytes("rt-1")))

        XCTAssertEqual(text(vault.read()), "rt-1")
    }

    /// Absent, not Unavailable, and the difference is what the caller may do next: a store that
    /// was never written may be written over, and one that merely could not be read may not.
    func testNothingWrittenIsAbsentRatherThanUnavailable() {
        XCTAssertTrue(KeychainVault(service: "\(service).empty").read() is StoredAbsent)
    }

    /// Nothing to delete is a wipe that happened. Reported as failure, the fresh-install guard
    /// above this would refuse to arm and would run again on every launch for ever.
    func testWipingWhatWasNeverThereSucceeds() {
        XCTAssertTrue(KeychainVault(service: "\(service).empty").wipe())
    }

    /// A second write updates rather than adding a second item: `SecItemAdd` over an existing
    /// account answers `errSecDuplicateItem`, and a vault ignoring that would keep handing back
    /// the session the patient signed out of.
    func testWritingTwiceUpdatesRatherThanDuplicates() {
        vault.write(bytes: bytes("rt-1"))
        vault.write(bytes: bytes("rt-2"))

        XCTAssertEqual(text(vault.read()), "rt-2")
    }

    func testWipingLeavesNothingBehindAndSaysSo() {
        vault.write(bytes: bytes("rt-1"))

        XCTAssertTrue(vault.wipe())

        XCTAssertTrue(vault.read() is StoredAbsent)
    }

    /// Two services are two stores: the session and the PKCE verifier live side by side, and one
    /// clearing the other would sign the patient out in the middle of accepting an invite.
    func testTwoServicesDoNotShareAStore() {
        let other = KeychainVault(service: "\(service).other")
        defer { other.wipe() }
        vault.write(bytes: bytes("session"))
        other.write(bytes: bytes("verifier"))

        XCTAssertEqual(text(vault.read()), "session")
        XCTAssertEqual(text(other.read()), "verifier")
    }

    /// The two attributes that keep the session on this device, read back off the item itself.
    /// No behavioural assertion above can see them: a store written `WhenUnlocked` round-trips
    /// identically and still locks a background token refresh out, and one written
    /// synchronizable travels to the patient's other devices while reading the same here.
    func testTheItemIsReadableAfterFirstUnlockAndOnlyOnThisDevice() {
        vault.write(bytes: bytes("rt-1"))

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecReturnAttributes as String: true,
        ]
        var found: CFTypeRef?
        XCTAssertEqual(SecItemCopyMatching(query as CFDictionary, &found), errSecSuccess)

        let attributes = found as? [String: Any]
        XCTAssertEqual(
            attributes?[kSecAttrAccessible as String] as? String,
            kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String
        )
        XCTAssertEqual(attributes?[kSecAttrSynchronizable as String] as? Bool, false)
    }

    private func bytes(_ value: String) -> KotlinByteArray {
        let utf8 = Array(value.utf8)
        let array = KotlinByteArray(size: Int32(utf8.count))
        for (index, byte) in utf8.enumerated() {
            array.set(index: Int32(index), value: Int8(bitPattern: byte))
        }
        return array
    }

    private func text(_ stored: Stored) -> String? {
        guard let array = (stored as? StoredPresent)?.bytes else { return nil }
        var utf8: [UInt8] = []
        for index in 0..<array.size {
            utf8.append(UInt8(bitPattern: array.get(index: index)))
        }
        return String(decoding: utf8, as: UTF8.self)
    }
}
