// Copyright 2026 beetz12. Licensed under Apache-2.0.
// care.com GraphQL operation documents captured from the authenticated web app.
// Hand-authored (not generator-emitted); safe across regen.

package cli

const careQSearchProvidersOp = "SearchProvidersChildCare"
const careQSearchProviders = `fragment CaregiverFragment on SearchProvidersSuccess {
  sourceType
  searchProvidersConnection {
    pageInfo {
      hasNextPage
      endCursor
      __typename
    }
    totalHits
    edges {
      node {
        ... on Caregiver {
          member {
            id
            legacyId
            imageURL
            displayName
            firstName
            lastName
            address {
              city
              state
              zip
              __typename
            }
            primaryService
            __typename
          }
          responseTime
          responsivenessScore
          hasCareCheck
          badges
          backgroundChecks {
            whenCompleted
            backgroundCheckName
            __typename
          }
          yearsOfExperience
          profileDataSource
          hiredByCounts {
            locality {
              hiredCount
              __typename
            }
            __typename
          }
          hiredTimes
          revieweeMetrics {
            ... on ReviewFailureResponse {
              message
              __typename
            }
            ... on RevieweeMetricsPayload {
              metrics {
                totalReviews
                averageRatings {
                  type
                  value
                  __typename
                }
                __typename
              }
              __typename
            }
            __typename
          }
          profiles {
            commonCaregiverProfile {
              id
              bio {
                experienceSummary
                __typename
              }
              __typename
            }
            childCareCaregiverProfile {
              rates {
                hourlyRate {
                  amount
                  __typename
                }
                numberOfChildren
                __typename
              }
              recurringRate {
                hourlyRateFrom {
                  amount
                  __typename
                }
                hourlyRateTo {
                  amount
                  __typename
                }
                __typename
              }
              __typename
            }
            petCareCaregiverProfile {
              serviceRates {
                duration
                rate {
                  amount
                  __typename
                }
                subtype
                __typename
              }
              recurringRate {
                hourlyRateFrom {
                  amount
                  __typename
                }
                hourlyRateTo {
                  amount
                  __typename
                }
                __typename
              }
              __typename
            }
            houseKeepingCaregiverProfile {
              recurringRate {
                hourlyRateFrom {
                  amount
                  __typename
                }
                hourlyRateTo {
                  amount
                  __typename
                }
                __typename
              }
              __typename
            }
            tutoringCaregiverProfile {
              recurringRate {
                hourlyRateFrom {
                  amount
                  __typename
                }
                hourlyRateTo {
                  amount
                  __typename
                }
                __typename
              }
              __typename
            }
            seniorCareCaregiverProfile {
              recurringRate {
                hourlyRateFrom {
                  amount
                  __typename
                }
                hourlyRateTo {
                  amount
                  __typename
                }
                __typename
              }
              __typename
            }
            __typename
          }
          featuredReview {
            description {
              displayText
              originalText
              __typename
            }
            reviewer {
              publicMemberInfo {
                firstName
                lastInitial
                imageURL
                __typename
              }
              __typename
            }
            __typename
          }
          isFavorite
          isAvailable
          __typename
        }
        ... on SearchProvidersNodeError {
          providerId
          message
          __typename
        }
        __typename
      }
      __typename
    }
    __typename
  }
  __typename
}

query SearchProvidersChildCare($input: SearchProvidersChildCareInput!) {
  searchProvidersChildCare(input: $input) {
    ... on SearchProvidersSuccess {
      ...CaregiverFragment
      __typename
    }
    ... on SearchProvidersError {
      message
      __typename
    }
    __typename
  }
}`

const careQGetCaregiverOp = "GetCaregiver"
const careQGetCaregiver = `query GetCaregiver($getCaregiverId: ID!, $serviceId: ServiceType, $shouldIncludeAllProfiles: Boolean, $shouldGetMarkedAsHired: Boolean!) {
  getCaregiver(
    id: $getCaregiverId
    serviceId: $serviceId
    shouldIncludeAllProfiles: $shouldIncludeAllProfiles
  ) {
    continuousBackgroundCheck {
      seeker {
        hasLimitReached
        subscriptionStatus
        __typename
      }
      hasActiveHit
      __typename
    }
    backgroundChecks {
      backgroundCheckName
      backgroundCheckStatus
      whenCompleted
      __typename
    }
    badges
    isAvailable
    distanceFromSeekerInMiles
    educationDegrees {
      currentlyAttending
      degreeYear
      educationLevel
      schoolName
      educationDetailsText
      __typename
    }
    hasCareCheck
    hasGrantedCriminalBGCAccess
    hasGrantedMvrBGCAccess
    hasGrantedPremierBGCAccess
    isPaymentReady
    isServiceReviewable
    hiredByCounts {
      locality {
        hiredCount
        __typename
      }
      __typename
    }
    hiredTimes
    revieweeMetrics {
      ... on ReviewFailureResponse {
        message
        __typename
      }
      ... on RevieweeMetricsPayload {
        metrics {
          totalReviews
          averageRatings {
            type
            value
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    isFavorite
    isMVREligible
    isVaccinated
    member {
      id
      lastName
      firstName
      gender
      hiResImageURL
      displayName
      email
      primaryService
      imageURL
      address {
        city
        state
        zip
        __typename
      }
      languages
      legacyId
      isPremium
      __typename
    }
    placeInfo {
      name
      __typename
    }
    profiles {
      serviceIds
      childCareCaregiverProfile {
        availabilityFrequency
        approvalStatus
        ageGroups
        availabilityFrequency
        bio {
          experienceSummary
          title
          aiAssistedBio
          __typename
        }
        childStaffRatio
        distanceWillingToTravel {
          unit
          value
          __typename
        }
        id
        maxAgeMonths
        minAgeMonths
        numberOfChildren
        offerings {
          ageRange {
            max {
              unit
              value
              __typename
            }
            min {
              unit
              value
              __typename
            }
            __typename
          }
          __typename
        }
        otherQualities
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        qualities {
          afterSchoolCare
          babysitter
          certifiedNursingAssistant
          certifiedRegisterNurse
          certifiedTeacher
          childDevelopmentAssociate
          comfortableWithPets
          cprTrained
          crn
          doesNotSmoke
          doula
          earlyChildDevelopmentCoursework
          earlyChildhoodEducation
          firstAidTraining
          mothersHelper
          nafccCertified
          nanny
          newbornCareSpecialist
          nightNanny
          ownTransportation
          specialNeedsCare
          trustlineCertifiedCalifornia
          __typename
        }
        recurringRate {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        rates {
          hourlyRate {
            amount
            currencyCode
            __typename
          }
          numberOfChildren
          isDefaulted
          __typename
        }
        supportedServices {
          carpooling
          craftAssistance
          errands
          groceryShopping
          laundryAssistance
          lightHousekeeping
          mealPreparation
          swimmingSupervision
          travel
          __typename
        }
        yearsOfExperience
        __typename
      }
      commonCaregiverProfile {
        id
        repeatClientsCount
        merchandizedJobInterests {
          companionCare
          dateNight
          lightCleaning
          mealPrepLaundry
          mover
          personalAssistant
          petHelp
          shopping
          transportation
          __typename
        }
        __typename
      }
      petCareCaregiverProfile {
        availabilityFrequency
        approvalStatus
        rates {
          activity
          activityRate {
            amount
            currencyCode
            __typename
          }
          activityRateUnit
          __typename
        }
        bio {
          experienceSummary
          title
          __typename
        }
        id
        numberOfPetsComfortableWith
        otherQualities
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        petSpecies {
          caresForAmphibians
          caresForBirds
          caresForCats
          caresForDogs
          caresForExoticPets
          caresForFarmAnimals
          caresForFish
          caresForHorses
          caresForMammals
          caresForOtherPets
          __typename
        }
        qualities {
          isBondedAndInsured
          isNappsCertified
          isPSAMemberAndIsInsured
          isRedCrossPetFirstAidCertified
          doesNotSmoke
          ownsTransportation
          isCertifiedAndInsured
          isExperiencedWithChallengingPets
          isExperiencedWithSeniorPets
          isExperiencedWithSpecialNeedsPets
          isExperiencedWithYoungPets
          __typename
        }
        recurringRate {
          hourlyRateFrom {
            currencyCode
            amount
            __typename
          }
          hourlyRateTo {
            currencyCode
            amount
            __typename
          }
          __typename
        }
        serviceRates {
          rate {
            amount
            currencyCode
            __typename
          }
          duration
          subtype
          __typename
        }
        supportedServices {
          administersMedicine
          boardsOvernight
          doesDailyFeeding
          doesHouseSitting
          doesPetDaycare
          doesPetSitting
          doesPetWalking
          groomsAnimals
          retrievesMail
          trainsDogs
          transportsPets
          watersPlants
          __typename
        }
        yearsOfExperience
        __typename
      }
      seniorCareCaregiverProfile {
        availabilityFrequency
        approvalStatus
        availabilityFrequency
        bio {
          experienceSummary
          title
          __typename
        }
        id
        otherQualities
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        qualities {
          alzheimersOrDementiaExperience
          certifiedNursingAssistant
          comfortableWithPets
          cprTrained
          doesNotSmoke
          firstAidTraining
          homeHealthAideExperience
          hospiceExperience
          licensedNurse
          medicalEquipmentExperience
          ownTransportation
          registeredNurse
          woundCare
          __typename
        }
        recurringRate {
          hourlyRateFrom {
            currencyCode
            amount
            __typename
          }
          hourlyRateTo {
            currencyCode
            amount
            __typename
          }
          __typename
        }
        supportedServices {
          visitingPhysician
          visitingNurse
          transportation
          specializedCare
          specialNeeds
          respiteCare
          personalCare
          mobilityAssistance
          medicalTransportation
          medicalManagement
          mealPreparation
          lightHousekeeping
          liveInHomeCare
          hospiceServices
          homeModification
          homeHealth
          helpStayingPhysicallyActive
          heavyLifting
          feeding
          errands
          dementia
          companionship
          bathing
          __typename
        }
        yearsOfExperience
        __typename
      }
      tutoringCaregiverProfile {
        availabilityFrequency
        approvalStatus
        availabilityFrequency
        bio {
          experienceSummary
          title
          __typename
        }
        id
        otherGeneralSubjects
        otherQualities
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        qualities {
          additionalDetails {
            doesNotSmoke
            isComfortableWithPets
            ownsTransportation
            __typename
          }
          professionalSkills {
            americanTutoringAssociationCertified
            certifiedTeacher
            __typename
          }
          __typename
        }
        supportedServices {
          tutorsInCenter
          tutorsInStudentsHome
          tutorsInTeachersHome
          tutorsOnline
          __typename
        }
        specificSubjects
        otherSpecificSubject
        yearsOfExperience
        __typename
      }
      houseKeepingCaregiverProfile {
        availabilityFrequency
        approvalStatus
        availabilityFrequency
        bio {
          experienceSummary
          title
          __typename
        }
        distanceWillingToTravel {
          unit
          value
          __typename
        }
        id
        otherQualities
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        qualities {
          comfortableWithPets
          doesNotSmoke
          ownTransportation
          providesEquipment
          providesSupplies
          __typename
        }
        recurringRate {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        schedule {
          endTime
          id
          ruleName
          rules
          startTime
          __typename
        }
        supportedServices {
          greenCleaning
          springSummerCleaning
          atticCleaning
          basementCleaning
          bathroomCleaning
          cabinetCleaning
          carpetCleaning
          changingBedLinens
          deepCleaning
          dishes
          dusting
          furnitureTreatment
          generalRoomCleaning
          houseSitting
          kitchenCleaning
          laundry
          moveOutCleaning
          organization
          ovenCleaning
          packingUnpacking
          petWasteCleanup
          plantCare
          refrigeratorCleaning
          standardCleaning
          surfacePolishing
          vacuumingOrMopping
          wallWashing
          windowWashing
          __typename
        }
        yearsOfExperience
        __typename
      }
      __typename
    }
    providerStatus
    responseRate
    responseTime
    signUpDate
    yearsOfExperience
    nonPrimaryImages
    isMarkedAsHired @include(if: $shouldGetMarkedAsHired)
    __typename
  }
}`

const careQGetCaregiverProfileOp = "getCaregiverProfile"
const careQGetCaregiverProfile = `query getCaregiverProfile($id: ID!) {
  getCaregiver(id: $id) {
    isMarkedAsHired
    member {
      id
      primaryService
      __typename
    }
    profiles {
      commonCaregiverProfile {
        id
        payRange {
          hourlyRateFrom {
            amount
            currencyCode
            __typename
          }
          hourlyRateTo {
            amount
            currencyCode
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    avgReviewRating
    yearsOfExperience
    __typename
  }
}`

const careQProviderOp = "Provider"
const careQProvider = `query Provider($providerId: ID!) {
  provider(id: $providerId) {
    ... on ProviderLookupSuccess {
      provider {
        publicId
        __typename
      }
      __typename
    }
    __typename
  }
}`

const careQMessageThreadOp = "GetMessageThread"
const careQMessageThread = `query GetMessageThread($userId: ID!, $otherUserId: ID!) {
  getMessageThread(userId: $userId, otherUserId: $otherUserId) {
    id
    __typename
  }
}`

const careQReviewsByRevieweeOp = "ReviewsByReviewee"
const careQReviewsByReviewee = `query ReviewsByReviewee($revieweeId: ID!, $revieweeType: ReviewInfoEntityType!, $careType: ReviewInfoCareType, $pageSize: Int, $pageToken: String) {
  reviewsByReviewee(
    revieweeId: $revieweeId
    revieweeType: $revieweeType
    careType: $careType
    pageSize: $pageSize
    pageToken: $pageToken
  ) {
    ... on ReviewsByRevieweePayload {
      __typename
      nextPageToken
      reviews {
        attributes {
          truthy
          type
          __typename
        }
        careType
        createTime
        deleteTime
        description {
          displayText
          originalText
          __typename
        }
        id
        languageCode
        originalSource
        ratings {
          type
          value
          __typename
        }
        retort {
          displayText
          originalText
          __typename
        }
        reviewee {
          id
          providerType
          type
          __typename
        }
        reviewer {
          imageURL
          publicMemberInfo {
            firstName
            lastInitial
            __typename
          }
          source
          type
          __typename
        }
        status
        updateSource
        updateTime
        verifiedByCare
        __typename
      }
    }
    ... on ReviewFailureResponse {
      message
      __typename
    }
    __typename
  }
}`

const careQJobApplicationsOp = "JobApplications"
const careQJobApplications = `query JobApplications($jobId: ID!, $sortBy: JobApplicationsSortBy, $max: Int, $jobApplicationFilter: SeekerJobApplicationFilter, $start: String, $includeRecentMessage: Boolean = false) {
  jobApplications: jobProfile(jobId: $jobId) {
    ... on JobProfileSuccess {
      job {
        id
        serviceType
        adultCareJob
        startDate
        applicationCounts {
          total
          __typename
        }
        applications(
          sortBy: $sortBy
          max: $max
          jobApplicationFilter: $jobApplicationFilter
          start: $start
        ) {
          ... on JobApplicationsConnection {
            edges {
              node {
                ...ApplicantFragment
                __typename
              }
              cursor
              __typename
            }
            filteredCount
            pageInfo {
              endCursor
              hasNextPage
              hasPreviousPage
              startCursor
              __typename
            }
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    __typename
  }
}

fragment ApplicantFragment on JobApplicationLinkage {
  jobApplicationId
  conversationId
  seekerInterest
  privateNote
  boostedApplication
  applicant {
    ... on Caregiver {
      yearsOfExperience
      member {
        imageURL
        id
        legacyId
        isPremium
        displayName
        address {
          city
          zip
          state
          __typename
        }
        __typename
      }
      badges
      hiredTimes
      isFavorite
      profileURL
      messageThreadId
      profiles {
        tutoringCaregiverProfile {
          id
          payRange {
            ...PayRangeFragment
            __typename
          }
          recurringRate {
            ...PayRangeFragment
            __typename
          }
          __typename
        }
        childCareCaregiverProfile {
          id
          payRange {
            ...PayRangeFragment
            __typename
          }
          recurringRate {
            ...PayRangeFragment
            __typename
          }
          __typename
        }
        houseKeepingCaregiverProfile {
          id
          payRange {
            ...PayRangeFragment
            __typename
          }
          recurringRate {
            ...PayRangeFragment
            __typename
          }
          __typename
        }
        petCareCaregiverProfile {
          id
          payRange {
            ...PayRangeFragment
            __typename
          }
          recurringRate {
            ...PayRangeFragment
            __typename
          }
          __typename
        }
        seniorCareCaregiverProfile {
          id
          payRange {
            ...PayRangeFragment
            __typename
          }
          recurringRate {
            ...PayRangeFragment
            __typename
          }
          __typename
        }
        __typename
      }
      revieweeMetrics {
        ... on RevieweeMetricsPayload {
          metrics {
            averageRatings {
              type
              value
              __typename
            }
            totalReviews
            __typename
          }
          __typename
        }
        __typename
      }
      mostRecentMessage @include(if: $includeRecentMessage) {
        ... on ConversationMessageSnippet {
          message
          truncatedMessage
          __typename
        }
        __typename
      }
      __typename
    }
    __typename
  }
  __typename
}

fragment PayRangeFragment on PayRange {
  hourlyRateFrom {
    amount
    __typename
  }
  __typename
}`

const careQJobsBySeekerOp = "jobsBySeekerUUID"
const careQJobsBySeeker = `query jobsBySeekerUUID($after: String, $jobFilter: JobFilter, $seekerUuid: ID!, $first: Int) {
  jobsBySeekerUUID(
    after: $after
    jobFilter: $jobFilter
    seekerUUID: $seekerUuid
    first: $first
  ) {
    ... on JobsConnection {
      edges {
        node {
          applicationCounts {
            total
            __typename
          }
          id
          serviceType
          title
          status
          startDate
          postDate
          statusLastModifiedDate
          applications(max: 3) {
            ... on JobApplicationsConnection {
              edges {
                node {
                  applicant {
                    ... on Caregiver {
                      revieweeMetrics {
                        ... on RevieweeMetricsPayload {
                          metrics {
                            totalReviews
                            averageRatings {
                              type
                              value
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        __typename
                      }
                      member {
                        id
                        id
                        imageURL
                        displayName
                        address {
                          city
                          state
                          zip
                          __typename
                        }
                        __typename
                      }
                      profiles {
                        childCareCaregiverProfile {
                          payRange {
                            hourlyRateFrom {
                              amount
                              currencyCode
                              __typename
                            }
                            __typename
                          }
                          rates {
                            hourlyRate {
                              amount
                              __typename
                            }
                            __typename
                          }
                          recurringRate {
                            hourlyRateFrom {
                              amount
                              __typename
                            }
                            hourlyRateTo {
                              amount
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        petCareCaregiverProfile {
                          payRange {
                            hourlyRateFrom {
                              amount
                              currencyCode
                              __typename
                            }
                            __typename
                          }
                          serviceRates {
                            rate {
                              amount
                              __typename
                            }
                            subtype
                            __typename
                          }
                          recurringRate {
                            hourlyRateFrom {
                              amount
                              __typename
                            }
                            hourlyRateTo {
                              amount
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        seniorCareCaregiverProfile {
                          payRange {
                            hourlyRateFrom {
                              amount
                              currencyCode
                              __typename
                            }
                            __typename
                          }
                          recurringRate {
                            hourlyRateFrom {
                              amount
                              __typename
                            }
                            hourlyRateTo {
                              amount
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        tutoringCaregiverProfile {
                          payRange {
                            hourlyRateFrom {
                              amount
                              currencyCode
                              __typename
                            }
                            __typename
                          }
                          recurringRate {
                            hourlyRateFrom {
                              amount
                              __typename
                            }
                            hourlyRateTo {
                              amount
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        houseKeepingCaregiverProfile {
                          payRange {
                            hourlyRateFrom {
                              amount
                              currencyCode
                              __typename
                            }
                            __typename
                          }
                          recurringRate {
                            hourlyRateFrom {
                              amount
                              __typename
                            }
                            hourlyRateTo {
                              amount
                              __typename
                            }
                            __typename
                          }
                          __typename
                        }
                        __typename
                      }
                      __typename
                    }
                    __typename
                  }
                  __typename
                }
                __typename
              }
              __typename
            }
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    __typename
  }
}`

const careQLoggedInUserOp = "loggedInUser"
const careQLoggedInUser = `query loggedInUser {
  loggedInUser {
    roles
    overallStatus
    legacyId
    subject
    __typename
  }
}`

const careQConversationsOp = "conversationMostRelevantCareNeeds"
const careQConversations = `query conversationMostRelevantCareNeeds($loggedInUserRole: LoggedInUserRole, $participantsMetadata: [ConversationParticipantMetadata!]!) {
  conversationMostRelevantCareNeeds(
    loggedInUserRole: $loggedInUserRole
    participantsMetadata: $participantsMetadata
  ) {
    error
    haveCompletedBooking
    participantId
    mostRelevantBooking {
      expiresAt
      status
      bookingRequestJob {
        id
        serviceType
        jobType
        careSubtype {
          ... on PetCareSubtypeWrapper {
            petCareSubtype
            __typename
          }
          __typename
        }
        recurrenceRules {
          rules
          startTime
          endTime
          __typename
        }
        serviceDetails {
          ... on ChildCareServiceDetails {
            children {
              name
              __typename
            }
            __typename
          }
          ... on PetCareServiceDetails {
            pets {
              ... on Cat {
                name
                __typename
              }
              ... on Dog {
                name
                __typename
              }
              __typename
            }
            __typename
          }
          __typename
        }
        timeBlock {
          end
          start
          __typename
        }
        __typename
      }
      slot {
        timeBlock {
          end
          start
          __typename
        }
        __typename
      }
      id
      __typename
    }
    mostRelevantInvitation {
      id
      bookingId
      expiresAt
      status
      subStatus
      caregiverTier
      bookingRequestJob {
        id
        serviceType
        jobType
        careSubtype {
          ... on PetCareSubtypeWrapper {
            petCareSubtype
            __typename
          }
          __typename
        }
        recurrenceRules {
          rules
          startTime
          endTime
          __typename
        }
        serviceDetails {
          ... on ChildCareServiceDetails {
            children {
              name
              __typename
            }
            __typename
          }
          ... on PetCareServiceDetails {
            pets {
              ... on Cat {
                name
                __typename
              }
              ... on Dog {
                name
                __typename
              }
              __typename
            }
            __typename
          }
          __typename
        }
        timeBlock {
          end
          start
          __typename
        }
        __typename
      }
      __typename
    }
    mostRelevantJob {
      ... on MostRelevantJob {
        job {
          id
          status
          serviceType
          startDate
          __typename
        }
        jobCtaStatus
        applicationID
        details {
          ... on JobDetails {
            oneTimeJob
            recurringDays {
              Friday
              Monday
              Saturday
              Sunday
              Thursday
              Tuesday
              Wednesday
              __typename
            }
            timeBlock {
              end
              start
              __typename
            }
            __typename
          }
          ... on JobDetailsError {
            errorMessage
            __typename
          }
          __typename
        }
        __typename
      }
      ... on JobError {
        id
        message
        __typename
      }
      __typename
    }
    __typename
  }
}`
