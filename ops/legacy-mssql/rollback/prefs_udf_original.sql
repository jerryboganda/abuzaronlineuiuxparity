CREATE FUNCTION DBO.Fn_GetPreference( @PrefName VARCHAR(100) )
RETURNS VARCHAR(200)
AS
BEGIN
/*
Author : Muhammad Azhar Inayat

Purpose : This function finds the preference name pass as parameter to it and
returns the string value of that function
*/
DECLARE @PrefValue VARCHAR(200)

SET @PrefValue = ISNULL((SELECT PrefValue FROM DBO.SoftwarePreferences WHERE LOWER(Name) = LOWER(@prefname)), '')

RETURN LTRIM(RTRIM(@PrefValue))
END
GO
CREATE FUNCTION DBO.Fn_Get_Int_Preference( @PrefName VARCHAR(100) )
RETURNS INT
AS
BEGIN
/*
Author : Muhammad Azhar Inayat

Purpose : This function finds the preference name pass as parameter to it and
returns the integer value of that function
*/
DECLARE @PrefValue INT

SET @PrefValue = CAST((SELECT LTRIM(RTRIM(PrefValue)) FROM DBO.SoftwarePreferences WHERE LOWER(Name) = LOWER(@prefname)) AS INT)

RETURN @PrefValue
END
GO
CREATE FUNCTION DBO.Fn_Get_DateTime_Preference( @PrefName VARCHAR(100) )
RETURNS DATETIME
AS
BEGIN
/*
Author : Muhammad Azhar Inayat

Purpose : This function finds the preference name pass as parameter to it and
returns the datetime value of that function
*/
DECLARE @PrefValue DATETIME

SET @PrefValue = CAST((SELECT LTRIM(RTRIM(PrefValue)) FROM DBO.SoftwarePreferences WHERE LOWER(Name) = LOWER(@prefname)) AS DATETIME)

RETURN @PrefValue
END
